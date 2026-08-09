package app

import (
	"errors"
	"fmt"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// ErrAccountNotFound is returned by AccountService.Get when no account
// matches the given ID.
var ErrAccountNotFound = errors.New("account not found")

// AccountService implements the read-only account queries needed by the
// CLI this sprint (RF-02, RF-03). Mutating operations (add/edit/remove)
// arrive in Sprint 2-3.
type AccountService struct {
	repo AccountRepository
}

// NewAccountService builds an AccountService backed by repo.
func NewAccountService(repo AccountRepository) *AccountService {
	return &AccountService{repo: repo}
}

// List returns every registered account (RF-02).
func (s *AccountService) List() ([]domain.Account, error) {
	return s.repo.List()
}

// Get returns the account identified by id, or ErrAccountNotFound (RF-03).
func (s *AccountService) Get(id string) (domain.Account, error) {
	account, err := s.repo.FindByID(id)
	if err != nil {
		return domain.Account{}, fmt.Errorf("look up account %q: %w", id, err)
	}
	if account == nil {
		return domain.Account{}, ErrAccountNotFound
	}
	return *account, nil
}
