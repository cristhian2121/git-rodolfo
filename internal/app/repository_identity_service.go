package app

import (
	"errors"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// CurrentStatus is what "git rodolfo current" reports (§13.8).
type CurrentStatus struct {
	// Managed is false when the repository has no rodolfo.account at all
	// — "not managed by Git Rodolfo," not an error.
	Managed bool
	// Account is the assigned account. Zero value if Managed is false, or
	// if AccountDeleted is true (rodolfo.account points at an account
	// that no longer exists).
	Account        domain.Account
	AccountDeleted bool
	AssignedID     string // set when AccountDeleted, so the caller can report which id is missing

	HasRemote bool
	RemoteURL string

	// Validation is only meaningful when Managed and !AccountDeleted.
	Validation domain.ValidationResult

	// EffectiveGitEmail is what commits would use absent Git Rodolfo —
	// shown when !Managed (§13.8's "It will use your global Git
	// identity").
	EffectiveGitEmail string
}

// RepositoryIdentityService implements "git rodolfo use" and "git rodolfo
// current" (RF-17, RF-18, RF-19, RF-20, §13.7, §13.8): it's bound to one
// repository's GitClient for its whole lifetime, matching how the CLI
// constructs one per invocation (the working directory the command runs
// in).
type RepositoryIdentityService struct {
	git       GitClient
	mechanism IdentityMechanism
	accounts  AccountRepository
}

// NewRepositoryIdentityService builds a RepositoryIdentityService bound to
// git (the current repository).
func NewRepositoryIdentityService(git GitClient, mechanism IdentityMechanism, accounts AccountRepository) *RepositoryIdentityService {
	return &RepositoryIdentityService{git: git, mechanism: mechanism, accounts: accounts}
}

// Use applies account's identity to the bound repository (RF-17, RF-18,
// RF-09). It works whether or not the repository has a remote (RF-19).
func (s *RepositoryIdentityService) Use(account domain.Account) error {
	return s.mechanism.Apply(s.git, account)
}

// Clear reverts every value Git Rodolfo may have set (RF-20).
func (s *RepositoryIdentityService) Clear() error {
	return s.mechanism.Clear(s.git)
}

// RemoteURL returns the repository's origin URL, or ("", false) if there
// is none (RF-19).
func (s *RepositoryIdentityService) RemoteURL() (string, bool, error) {
	url, err := s.git.GetRemoteURL("origin")
	if errors.Is(err, ErrNoRemote) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return url, true, nil
}

// Current computes the bound repository's status (§13.8).
func (s *RepositoryIdentityService) Current() (CurrentStatus, error) {
	var status CurrentStatus

	if url, hasRemote, err := s.RemoteURL(); err != nil {
		return CurrentStatus{}, err
	} else {
		status.HasRemote = hasRemote
		status.RemoteURL = url
	}

	accountID, err := s.git.GetLocalConfig("rodolfo.account")
	if errors.Is(err, ErrConfigKeyNotSet) || accountID == "" {
		email, err := s.git.GetEffectiveConfig("user.email")
		if err != nil && !errors.Is(err, ErrConfigKeyNotSet) {
			return CurrentStatus{}, err
		}
		status.EffectiveGitEmail = email
		return status, nil
	}
	if err != nil {
		return CurrentStatus{}, err
	}
	status.Managed = true

	account, err := s.accounts.FindByID(accountID)
	if err != nil {
		return CurrentStatus{}, err
	}
	if account == nil {
		status.AccountDeleted = true
		status.AssignedID = accountID
		return status, nil
	}
	status.Account = *account

	validation, err := s.mechanism.Verify(s.git, *account)
	if err != nil {
		return CurrentStatus{}, err
	}
	status.Validation = validation

	return status, nil
}
