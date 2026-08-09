// Package infra contains the real (non-fake) implementations of the ports
// declared in internal/app.
package infra

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/atomicfile"
	"github.com/lean-tech/git-rodolfo/internal/infra/filelock"
)

// ConfigRepository implements app.AccountRepository on top of
// ~/.config/git-rodolfo/config.json (PRD §17), guarding every write with an
// exclusive file lock (RNF-06) and writing atomically (RNF-05).
type ConfigRepository struct {
	path string
}

// NewConfigRepository returns a repository backed by the config file at path.
func NewConfigRepository(path string) *ConfigRepository {
	return &ConfigRepository{path: path}
}

// DefaultConfigPath resolves ~/.config/git-rodolfo/config.json, honoring
// XDG_CONFIG_HOME when set (PRD §17), or %USERPROFILE%\.git-rodolfo\config.json
// on Windows (PRD 2 §13, RF-44).
func DefaultConfigPath() (string, error) {
	return defaultConfigPath(runtime.GOOS)
}

// defaultConfigPath takes goos as a parameter so the Windows branch is
// testable without actually running on Windows.
func defaultConfigPath(goos string) (string, error) {
	if goos == "windows" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		return filepath.Join(home, ".git-rodolfo", "config.json"), nil
	}

	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "git-rodolfo", "config.json"), nil
}

func (r *ConfigRepository) lockPath() string {
	return r.path + ".lock"
}

// read loads the current config. A missing file is not an error: it means
// a fresh install with no accounts yet.
func (r *ConfigRepository) read() (domain.GlobalConfig, error) {
	data, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return domain.NewGlobalConfig(), nil
	}
	if err != nil {
		return domain.GlobalConfig{}, fmt.Errorf("read %s: %w", r.path, err)
	}

	var cfg domain.GlobalConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return domain.GlobalConfig{}, fmt.Errorf("parse %s: %w", r.path, err)
	}
	return cfg, nil
}

func (r *ConfigRepository) write(cfg domain.GlobalConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := atomicfile.Write(r.path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", r.path, err)
	}
	return nil
}

// withLock runs fn against the current config under an exclusive lock, then
// persists whatever fn leaves in cfg. Two processes calling withLock at the
// same time serialize instead of racing: the second one's read happens only
// after the first one's write has completed and the lock is released.
func (r *ConfigRepository) withLock(fn func(cfg *domain.GlobalConfig) error) error {
	// On a fresh install neither the config directory nor the lock file
	// exist yet; the lock file needs the directory to already be there.
	if err := os.MkdirAll(filepath.Dir(r.path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	lock, err := filelock.Acquire(r.lockPath())
	if err != nil {
		return fmt.Errorf("acquire config lock: %w", err)
	}
	defer lock.Unlock()

	cfg, err := r.read()
	if err != nil {
		return err
	}
	if err := fn(&cfg); err != nil {
		return err
	}
	return r.write(cfg)
}

// List returns every registered account (RF-02).
func (r *ConfigRepository) List() ([]domain.Account, error) {
	cfg, err := r.read()
	if err != nil {
		return nil, err
	}
	return cfg.Accounts, nil
}

// FindByID returns the account with the given ID, or (nil, nil) if none
// matches — "not found" is not an error condition for this method; callers
// decide how to report it to the user.
func (r *ConfigRepository) FindByID(id string) (*domain.Account, error) {
	cfg, err := r.read()
	if err != nil {
		return nil, err
	}
	for i := range cfg.Accounts {
		if cfg.Accounts[i].ID == id {
			found := cfg.Accounts[i]
			return &found, nil
		}
	}
	return nil, nil
}

// FindByFingerprint returns the account whose SSH key has the given public
// key fingerprint, or (nil, nil) if none matches (used to detect a key
// already claimed by another account, RF-07).
func (r *ConfigRepository) FindByFingerprint(fp string) (*domain.Account, error) {
	cfg, err := r.read()
	if err != nil {
		return nil, err
	}
	for i := range cfg.Accounts {
		if cfg.Accounts[i].PublicKeyFingerprint == fp {
			found := cfg.Accounts[i]
			return &found, nil
		}
	}
	return nil, nil
}

// Save inserts a new account. It fails if an account with the same ID
// already exists — use Update to modify one.
func (r *ConfigRepository) Save(account domain.Account) error {
	return r.withLock(func(cfg *domain.GlobalConfig) error {
		for _, a := range cfg.Accounts {
			if a.ID == account.ID {
				return fmt.Errorf("account %q already exists", account.ID)
			}
		}
		cfg.Accounts = append(cfg.Accounts, account)
		return nil
	})
}

// Update replaces an existing account. It fails if no account with that ID
// exists — use Save to create one.
func (r *ConfigRepository) Update(account domain.Account) error {
	return r.withLock(func(cfg *domain.GlobalConfig) error {
		for i, a := range cfg.Accounts {
			if a.ID == account.ID {
				cfg.Accounts[i] = account
				return nil
			}
		}
		return fmt.Errorf("account %q not found", account.ID)
	})
}

// Delete removes the account with the given ID.
func (r *ConfigRepository) Delete(id string) error {
	return r.withLock(func(cfg *domain.GlobalConfig) error {
		for i, a := range cfg.Accounts {
			if a.ID == id {
				cfg.Accounts = append(cfg.Accounts[:i], cfg.Accounts[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("account %q not found", id)
	})
}
