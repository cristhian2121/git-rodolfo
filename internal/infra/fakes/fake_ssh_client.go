package fakes

import (
	"fmt"
	"os"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// FakeSSHClient is an in-memory app.SSHClient double. Keys is keyed by
// private key path; tests seed it to control ValidateKey/fingerprint/
// passphrase/connection-test behavior without touching real files or ssh.
type FakeSSHClient struct {
	Keys          map[string]FakeKey
	GeneratedKeys []GeneratedKey
	GenerateErr   error
	// GenerateFingerprint is the fingerprint recorded for a key created by
	// GenerateKey, standing in for what real ssh-keygen output would be.
	GenerateFingerprint string
	DeletedKeys         []string
	DeleteErr           error
}

// FakeKey is the seeded state of one SSH key.
type FakeKey struct {
	Valid         bool
	Fingerprint   string
	HasPassphrase bool
	AuthResult    domain.AuthResult
	AuthErr       error
	// PublicKeyContent stands in for the real public key line; if empty,
	// PublicKeyContent() synthesizes one from the path.
	PublicKeyContent string
	// Permissions defaults to 0o600 (secure) when zero.
	Permissions os.FileMode
}

// GeneratedKey records a single GenerateKey invocation for assertions.
type GeneratedKey struct {
	Path, Comment             string
	WithPassphrase, Overwrite bool
}

// NewFakeSSHClient returns an empty FakeSSHClient ready to use.
func NewFakeSSHClient() *FakeSSHClient {
	return &FakeSSHClient{Keys: map[string]FakeKey{}}
}

func (f *FakeSSHClient) GenerateKey(path, comment string, withPassphrase, overwrite bool) error {
	f.GeneratedKeys = append(f.GeneratedKeys, GeneratedKey{path, comment, withPassphrase, overwrite})
	if _, exists := f.Keys[path]; exists && !overwrite {
		return app.ErrKeyFileExists
	}
	if f.GenerateErr != nil {
		return f.GenerateErr
	}
	f.Keys[path] = FakeKey{Valid: true, HasPassphrase: withPassphrase, Fingerprint: f.GenerateFingerprint}
	return nil
}

func (f *FakeSSHClient) ValidateKey(path string) error {
	key, ok := f.Keys[path]
	if !ok || !key.Valid {
		return fmt.Errorf("invalid or missing key: %s", path)
	}
	return nil
}

func (f *FakeSSHClient) PublicKeyFingerprint(path string) (string, error) {
	key, ok := f.Keys[path]
	if !ok {
		return "", fmt.Errorf("unknown key: %s", path)
	}
	return key.Fingerprint, nil
}

func (f *FakeSSHClient) KeyHasPassphrase(path string) (bool, error) {
	key, ok := f.Keys[path]
	if !ok {
		return false, fmt.Errorf("unknown key: %s", path)
	}
	return key.HasPassphrase, nil
}

func (f *FakeSSHClient) TestConnectionWithKey(host, keyPath string) (domain.AuthResult, error) {
	key, ok := f.Keys[keyPath]
	if !ok {
		return domain.AuthResult{}, fmt.Errorf("unknown key: %s", keyPath)
	}
	return key.AuthResult, key.AuthErr
}

func (f *FakeSSHClient) PublicKeyContent(path string) (string, error) {
	key, ok := f.Keys[path]
	if !ok {
		return "", fmt.Errorf("unknown key: %s", path)
	}
	if key.PublicKeyContent != "" {
		return key.PublicKeyContent, nil
	}
	return "ssh-ed25519 FAKEKEY " + path, nil
}

func (f *FakeSSHClient) KeyFilePermissions(path string) (os.FileMode, error) {
	key, ok := f.Keys[path]
	if !ok {
		return 0, fmt.Errorf("unknown key: %s", path)
	}
	if key.Permissions == 0 {
		return 0o600, nil
	}
	return key.Permissions, nil
}

func (f *FakeSSHClient) DeleteKey(path string) error {
	if _, ok := f.Keys[path]; !ok {
		return fmt.Errorf("unknown key: %s", path)
	}
	if f.DeleteErr != nil {
		return f.DeleteErr
	}
	delete(f.Keys, path)
	f.DeletedKeys = append(f.DeletedKeys, path)
	return nil
}
