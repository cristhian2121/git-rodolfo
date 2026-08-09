package app

import (
	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// AccountRemoveService implements RF-05/RF-08 (§13.5): removing the local
// profile and, independently and only on explicit request, deleting the
// key files. The two are separate operations on purpose — if key deletion
// fails, the already-removed profile stays removed rather than being
// rolled back into a half-done state.
type AccountRemoveService struct {
	repo AccountRepository
	ssh  SSHClient
}

// NewAccountRemoveService builds an AccountRemoveService.
func NewAccountRemoveService(repo AccountRepository, ssh SSHClient) *AccountRemoveService {
	return &AccountRemoveService{repo: repo, ssh: ssh}
}

// RemoveProfile deletes the account's local profile and returns the
// removed account (so the caller can decide what to do about its key).
func (s *AccountRemoveService) RemoveProfile(id string) (domain.Account, error) {
	account, err := s.repo.FindByID(id)
	if err != nil {
		return domain.Account{}, err
	}
	if account == nil {
		return domain.Account{}, ErrAccountNotFound
	}
	if err := s.repo.Delete(id); err != nil {
		return domain.Account{}, err
	}
	return *account, nil
}

// KeyStillUsedByAnotherAccount reports whether account's key fingerprint
// is still claimed by a different, still-registered account. RF-07 already
// prevents two accounts from sharing a key going forward, but this is
// checked defensively before ever offering to delete key files (§13.5:
// "if the key is associated with another registered account, deletion is
// not offered").
func (s *AccountRemoveService) KeyStillUsedByAnotherAccount(account domain.Account) (bool, error) {
	if account.PublicKeyFingerprint == "" {
		return false, nil
	}
	other, err := s.repo.FindByFingerprint(account.PublicKeyFingerprint)
	if err != nil {
		return false, err
	}
	return other != nil && other.ID != account.ID, nil
}

// DeleteKeyFiles deletes account's private and public key files. Callers
// must obtain explicit confirmation first (RF-08) and should have already
// checked KeyStillUsedByAnotherAccount.
func (s *AccountRemoveService) DeleteKeyFiles(account domain.Account) error {
	if account.PrivateKeyPath == "" {
		return nil
	}
	return s.ssh.DeleteKey(account.PrivateKeyPath)
}
