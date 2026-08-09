package app

import (
	"errors"
	"fmt"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// IdentityMechanism applies, clears and verifies the identity forced onto
// a repository (PRD §12.2, §19.3). It's the seam that isolates the
// architecture decision in §12.2: nothing else in the application knows
// core.sshCommand is how identity gets enforced, so a future alias-based
// mechanism (§26) only needs a second implementation of this interface.
//
// Each method takes the GitClient bound to the target repository, rather
// than the mechanism holding one itself: CloneService needs to apply an
// identity to a repository that doesn't exist yet when the clone starts,
// so a single mechanism value has to work across different bound clients
// over its lifetime.
type IdentityMechanism interface {
	Apply(git GitClient, account domain.Account) error
	Clear(git GitClient) error
	Verify(git GitClient, account domain.Account) (domain.ValidationResult, error)
}

// SSHCommandMechanism is the MVP's only IdentityMechanism (§12.2's
// decision): it forces the account's key via a repository-local
// core.sshCommand, never touching ~/.ssh/config or the remote URL.
type SSHCommandMechanism struct{}

// NewSSHCommandMechanism builds an SSHCommandMechanism.
func NewSSHCommandMechanism() *SSHCommandMechanism {
	return &SSHCommandMechanism{}
}

// sshCommandFor renders the core.sshCommand value for account's key
// (§12.2): the path is quoted to tolerate spaces, and IdentitiesOnly=yes
// is what actually prevents ssh-agent from offering a different loaded key.
func sshCommandFor(account domain.Account) string {
	return fmt.Sprintf("ssh -i '%s' -o IdentitiesOnly=yes", account.PrivateKeyPath)
}

// Apply sets user.name, user.email, rodolfo.account and — when account has
// a key — core.sshCommand, all local to the repository (RF-09, RF-17,
// RF-18).
func (m *SSHCommandMechanism) Apply(git GitClient, account domain.Account) error {
	if err := git.SetLocalConfig("user.name", account.GitName); err != nil {
		return err
	}
	if err := git.SetLocalConfig("user.email", account.GitEmail); err != nil {
		return err
	}
	if err := git.SetLocalConfig("rodolfo.account", account.ID); err != nil {
		return err
	}
	if account.PrivateKeyPath == "" {
		return nil
	}
	// Defense in depth: AccountAddService/AccountEditService already
	// reject an unsafe path before it's ever saved, but Apply is where
	// the path actually gets embedded in a single-quoted shell string
	// (sshCommandFor) — checking again here means this can't regress
	// through some future or overlooked caller.
	if err := validateKeyPath(account.PrivateKeyPath); err != nil {
		return err
	}
	return git.SetLocalConfig("core.sshCommand", sshCommandFor(account))
}

// Clear removes every value Apply may have set (RF-20). It's safe to call
// on a repository Git Rodolfo never touched — unsetting an absent key is a
// no-op, not an error (RF-04's / RF-20's idempotence).
func (m *SSHCommandMechanism) Clear(git GitClient) error {
	for _, key := range []string{"user.name", "user.email", "rodolfo.account", "core.sshCommand"} {
		if err := git.UnsetLocalConfig(key); err != nil {
			return fmt.Errorf("clear %s: %w", key, err)
		}
	}
	return nil
}

// Verify reports whether the repository's local config actually matches
// account's identity, used by "git rodolfo current" to detect drift
// (§13.8) and after account edit to warn about repos left stale.
func (m *SSHCommandMechanism) Verify(git GitClient, account domain.Account) (domain.ValidationResult, error) {
	result := domain.ValidationResult{OK: true}

	check := func(label, key, want string) error {
		got, err := git.GetLocalConfig(key)
		if err != nil && !errors.Is(err, ErrConfigKeyNotSet) {
			return err
		}
		if got != want {
			result.OK = false
			result.Issues = append(result.Issues, fmt.Sprintf("%s does not match the assigned account", label))
		}
		return nil
	}

	if err := check("user.name", "user.name", account.GitName); err != nil {
		return domain.ValidationResult{}, err
	}
	if err := check("user.email", "user.email", account.GitEmail); err != nil {
		return domain.ValidationResult{}, err
	}
	if err := check("rodolfo.account", "rodolfo.account", account.ID); err != nil {
		return domain.ValidationResult{}, err
	}

	if account.PrivateKeyPath != "" {
		if err := check("SSH key", "core.sshCommand", sshCommandFor(account)); err != nil {
			return domain.ValidationResult{}, err
		}
	}

	return result, nil
}
