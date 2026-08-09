// Package app is the application layer (PRD §19.2): it orchestrates domain
// objects through the ports declared below. Sprint 1 defines all five
// infrastructure ports so every later sprint's business logic can be built
// and tested against fakes before any real Git/SSH/GitHub code exists
// (RNF-09) — only ConfigRepository (AccountRepository) has a real
// implementation so far, in internal/infra.
package app

import (
	"errors"
	"os"
	"strings"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// ErrKeyFileExists is returned by SSHClient.GenerateKey when path already
// exists and overwrite was false (RF-06: never overwrite silently).
var ErrKeyFileExists = errors.New("key file already exists")

// ErrConfigKeyNotSet is returned by GitClient.GetLocalConfig when the key
// simply isn't set on the repository — not an execution error.
var ErrConfigKeyNotSet = errors.New("config key not set")

// ErrNoRemote is returned by GitClient.GetRemoteURL when the named remote
// doesn't exist — expected and handled, not an execution error (RF-19:
// use/current must work on a repo with no origin at all).
var ErrNoRemote = errors.New("remote not configured")

// GitCommandError wraps a failed git invocation with its full output, so
// ErrorTranslator can pattern-match the real text (RF-21) instead of
// losing it to a truncated error message.
type GitCommandError struct {
	Output string
	Err    error
}

func (e *GitCommandError) Error() string {
	line := e.Output
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	return line
}

func (e *GitCommandError) Unwrap() error { return e.Err }

// GitClient wraps the subset of Git plumbing Git Rodolfo needs (PRD §19.3).
// A GitClient is bound to one working directory (how the real adapter is
// constructed); IsRepository is the one path-taking exception, used to
// check an arbitrary directory (e.g. a clone destination) before or
// instead of binding to it.
type GitClient interface {
	IsRepository(path string) bool
	// CloneWithSSHCommand clones remoteURL into destination. An empty
	// sshCommand clones without forcing any identity (RF-16's "keep
	// HTTPS" path).
	CloneWithSSHCommand(remoteURL, destination, sshCommand string) error
	SetLocalConfig(key, value string) error
	// GetLocalConfig returns ErrConfigKeyNotSet if key isn't set locally —
	// distinct from a real execution error.
	GetLocalConfig(key string) (string, error)
	// GetEffectiveConfig resolves key the way Git itself would (local,
	// falling through to global/system), used to show what identity
	// would apply to a repository Git Rodolfo doesn't manage (§13.8).
	GetEffectiveConfig(key string) (string, error)
	UnsetLocalConfig(key string) error
	// GetRemoteURL returns ErrNoRemote if remote doesn't exist.
	GetRemoteURL(remote string) (string, error)
}

// SSHClient wraps key generation, validation and connection testing.
type SSHClient interface {
	// GenerateKey creates a new Ed25519 key pair at path. If a file already
	// exists there and overwrite is false, it returns ErrKeyFileExists
	// without touching anything (RF-06). It never reads or stores a
	// passphrase (RF-11): when withPassphrase is true, the underlying
	// ssh-keygen prompts the caller's terminal directly.
	GenerateKey(path, comment string, withPassphrase, overwrite bool) error
	ValidateKey(path string) error
	PublicKeyFingerprint(path string) (string, error)
	KeyHasPassphrase(path string) (bool, error)
	TestConnectionWithKey(host, keyPath string) (domain.AuthResult, error)
	// PublicKeyContent returns the public key line (e.g. "ssh-ed25519
	// AAAA... comment") for the private key at path — what gets registered
	// on GitHub or copied to the clipboard.
	PublicKeyContent(path string) (string, error)
	// DeleteKey removes the private key at path and its ".pub" file
	// (RF-08). It's only ever called after explicit user confirmation.
	DeleteKey(path string) error
	// KeyFilePermissions returns path's file mode bits (e.g. 0o600), used
	// by "doctor" to flag insecure private key permissions (§13.10 #3).
	KeyFilePermissions(path string) (os.FileMode, error)
	// ScanKeys lists every private key in dir that has a matching public
	// key file and passes validation (RF-40, RF-42), so "account add" can
	// offer them in its selector. Strictly read-only: it never creates,
	// moves or modifies anything in dir. A missing dir is not an error —
	// it just yields no candidates.
	ScanKeys(dir string) ([]domain.SSHKeyCandidate, error)
}

// SSHAgentAdapter wraps the running ssh-agent, if any.
type SSHAgentAdapter interface {
	IsRunning() bool
	IsKeyLoaded(keyPath string) (bool, error)
	AddKey(keyPath string, useKeychain bool) error
}

// AccountRepository persists Account records (PRD §17, §18).
type AccountRepository interface {
	List() ([]domain.Account, error)
	FindByID(id string) (*domain.Account, error)
	FindByFingerprint(fp string) (*domain.Account, error)
	Save(account domain.Account) error
	Update(account domain.Account) error
	Delete(id string) error
}

// ProviderClient wraps GitHub-specific operations that go through GitHub
// CLI or the GitHub API.
type ProviderClient interface {
	VerifyAuthentication(account domain.Account) (domain.AuthResult, error)
	ActiveCLIUsername() (string, error)
	RegisterPublicKey(account domain.Account, publicKey string) error
}

// EnvironmentInspector reports on the local system for "doctor"'s
// environment-level validations (§13.10 #1 and #8).
type EnvironmentInspector interface {
	// GitVersion returns the installed git's version string (e.g.
	// "2.43.0"), or an error if git isn't found.
	GitVersion() (string, error)
	SSHInstalled() bool
	// Getenv looks up an environment variable; used to detect
	// GIT_SSH_COMMAND overriding core.sshCommand (§12.2, §13.10 #8).
	Getenv(key string) string
}

// ReleaseAsset is one downloadable file attached to a GitHub release.
type ReleaseAsset struct {
	Name        string
	DownloadURL string
}

// ReleaseInfo is the subset of a GitHub release the "update" command
// needs: its tag and the assets available for it.
type ReleaseInfo struct {
	TagName string
	Assets  []ReleaseAsset
}

// ReleaseFetcher looks up release metadata for "git-rodolfo update"
// (explicitly user-requested, so it doesn't run afoul of RNF-08's "no
// telemetry" rule — it only ever runs when the user types the command).
type ReleaseFetcher interface {
	// Latest returns the most recent published release.
	Latest() (ReleaseInfo, error)
	// ByTag returns the release tagged exactly tag (e.g. "v0.2.0").
	ByTag(tag string) (ReleaseInfo, error)
}

// AssetDownloader fetches a release asset's raw bytes (the release
// tarball) given the URL ReleaseFetcher reported for it.
type AssetDownloader interface {
	Download(url string) ([]byte, error)
}

// SelfUpdater replaces the currently running executable with newBinary's
// bytes (UpdateService.Update already extracted it from the release
// tarball). Only ever called after the user has explicitly confirmed.
type SelfUpdater interface {
	Replace(newBinary []byte) error
}
