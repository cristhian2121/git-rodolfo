package app

import (
	"fmt"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// ErrUnsupportedHost is RF-25's clone-time rejection: the MVP only
// supports github.com.
type ErrUnsupportedHost struct {
	Host   string
	Remote string
}

func (e *ErrUnsupportedHost) Error() string {
	return fmt.Sprintf("unsupported host %q (only github.com is supported)", e.Host)
}

// githubHostname is the only host the MVP accepts (PRD §8).
const githubHostname = "github.com"

// CloneService implements "git rodolfo clone" (RF-15, RF-16, RF-17, RF-18,
// RF-25, §13.6).
type CloneService struct {
	// git is used only for CloneWithSSHCommand, which doesn't depend on
	// which directory the instance is bound to.
	git GitClient
	// newGitForDir builds a GitClient bound to the freshly cloned
	// directory, so the identity mechanism can configure it after the
	// clone (the destination doesn't exist before then).
	newGitForDir func(dir string) GitClient
	mechanism    IdentityMechanism
}

// NewCloneService builds a CloneService.
func NewCloneService(git GitClient, newGitForDir func(dir string) GitClient, mechanism IdentityMechanism) *CloneService {
	return &CloneService{git: git, newGitForDir: newGitForDir, mechanism: mechanism}
}

// ValidateHost is RF-25 for clone: only github.com is accepted, and no
// partial configuration happens if it isn't (§21 "El proveedor no está
// soportado").
func ValidateHost(u domain.RepositoryURL, rawRemote string) error {
	if u.Host != githubHostname {
		return &ErrUnsupportedHost{Host: u.Host, Remote: rawRemote}
	}
	return nil
}

// Clone clones u into destination and configures account's identity
// locally. If forceSSH is false (RF-16's "keep HTTPS" choice), the clone
// runs without core.sshCommand and Apply skips it too — authentication is
// left to the user's credential helper, as the tool must warn about
// separately.
func (s *CloneService) Clone(u domain.RepositoryURL, destination string, account domain.Account, forceSSH bool) error {
	sshCommand := ""
	applyAccount := account
	if forceSSH {
		sshCommand = sshCommandFor(account)
	} else {
		applyAccount.PrivateKeyPath = "" // tells Apply to skip core.sshCommand
	}

	if err := s.git.CloneWithSSHCommand(u.String(), destination, sshCommand); err != nil {
		return err
	}

	repoGit := s.newGitForDir(destination)
	return s.mechanism.Apply(repoGit, applyAccount)
}
