package app

import (
	"time"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// AccountEditInput carries the fields to change for "account edit"
// (RF-04, §13.3). An empty string means "leave this field unchanged" —
// none of these fields are legitimately empty on a valid account.
type AccountEditInput struct {
	ID               string
	DisplayName      string
	GitName          string
	GitEmail         string
	ProviderUsername string
	// NewKeyPath associates a different existing key (validated and
	// duplicate-checked exactly like account add's "use an existing key").
	// Generating a brand new key during edit isn't supported — that's what
	// account remove + account add is for.
	NewKeyPath string
}

// AccountEditResult reports what changed, so the caller can render the
// before/after summary and decide what else to show or offer.
type AccountEditResult struct {
	Before domain.Account
	After  domain.Account
	// RepoConfigAffected is true when a field written into a repository's
	// .git/config (commit name/email or the key) changed — repositories
	// configured with this account need "git rodolfo use" re-run to pick
	// it up (Sprint 4 delivers "use"; this sprint only detects and warns).
	RepoConfigAffected bool
	// OldKeyPath is set when the key changed and there was a previous one,
	// which — because RF-07 already guarantees keys aren't shared between
	// accounts — is now unused by any account.
	OldKeyPath string
}

// AccountEditService implements RF-04.
type AccountEditService struct {
	repo AccountRepository
	ssh  SSHClient
	now  func() time.Time
}

// NewAccountEditService builds an AccountEditService. now defaults to
// time.Now if nil.
func NewAccountEditService(repo AccountRepository, ssh SSHClient, now func() time.Time) *AccountEditService {
	if now == nil {
		now = time.Now
	}
	return &AccountEditService{repo: repo, ssh: ssh, now: now}
}

// Edit previews and immediately commits input's changes — a convenience
// for callers (like --non-interactive) that don't need to show the diff
// before applying it. Interactive callers should use Preview then Commit
// instead, so they can confirm in between.
func (s *AccountEditService) Edit(input AccountEditInput) (AccountEditResult, error) {
	result, err := s.Preview(input)
	if err != nil {
		return AccountEditResult{}, err
	}
	if err := s.Commit(result.After); err != nil {
		return AccountEditResult{}, err
	}
	return result, nil
}

// Commit persists an account produced by Preview. Split out so callers can
// show a before/after diff and ask for confirmation between the two.
func (s *AccountEditService) Commit(after domain.Account) error {
	return s.repo.Update(after)
}

// Preview computes what input would change, including running the same
// key validation and duplicate-fingerprint check Edit does, but never
// writes anything — callers can show Before/After and only call Commit if
// the user confirms.
func (s *AccountEditService) Preview(input AccountEditInput) (AccountEditResult, error) {
	existing, err := s.repo.FindByID(input.ID)
	if err != nil {
		return AccountEditResult{}, err
	}
	if existing == nil {
		return AccountEditResult{}, ErrAccountNotFound
	}
	before := *existing
	after := *existing

	if input.DisplayName != "" {
		after.DisplayName = input.DisplayName
	}
	if input.GitName != "" {
		after.GitName = input.GitName
	}
	if input.GitEmail != "" {
		after.GitEmail = input.GitEmail
	}
	if input.ProviderUsername != "" {
		after.ProviderUsername = input.ProviderUsername
	}

	oldKeyPath := ""
	if input.NewKeyPath != "" && input.NewKeyPath != after.PrivateKeyPath {
		if err := validateKeyPath(input.NewKeyPath); err != nil {
			return AccountEditResult{}, err
		}
		if err := s.ssh.ValidateKey(input.NewKeyPath); err != nil {
			return AccountEditResult{}, err
		}
		fp, err := s.ssh.PublicKeyFingerprint(input.NewKeyPath)
		if err != nil {
			return AccountEditResult{}, err
		}
		if dup, err := s.repo.FindByFingerprint(fp); err != nil {
			return AccountEditResult{}, err
		} else if dup != nil && dup.ID != after.ID {
			return AccountEditResult{}, &KeyAlreadyInUseError{
				KeyPath:          input.NewKeyPath,
				OtherAccountName: dup.DisplayName,
				RequestedForName: after.DisplayName,
			}
		}

		oldKeyPath = after.PrivateKeyPath
		after.PrivateKeyPath = input.NewKeyPath
		after.PublicKeyPath = input.NewKeyPath + ".pub"
		after.PublicKeyFingerprint = fp
		// The new key hasn't been verified under this account yet.
		after.AuthenticationStatus = domain.AuthenticationStatusUnverified
		after.LastVerifiedAt = nil
	}

	after.UpdatedAt = s.now()

	repoConfigAffected := before.GitName != after.GitName ||
		before.GitEmail != after.GitEmail ||
		before.PrivateKeyPath != after.PrivateKeyPath

	return AccountEditResult{
		Before:             before,
		After:              after,
		RepoConfigAffected: repoConfigAffected,
		OldKeyPath:         oldKeyPath,
	}, nil
}
