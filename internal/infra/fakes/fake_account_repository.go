package fakes

import (
	"fmt"

	"github.com/lean-tech/git-rodolfo/internal/domain"
)

// FakeAccountRepository is an in-memory app.AccountRepository double, used
// to test AccountService and the CLI layer without touching disk.
type FakeAccountRepository struct {
	Accounts []domain.Account
}

// NewFakeAccountRepository returns a repository seeded with the given accounts.
func NewFakeAccountRepository(accounts ...domain.Account) *FakeAccountRepository {
	return &FakeAccountRepository{Accounts: accounts}
}

func (f *FakeAccountRepository) List() ([]domain.Account, error) {
	return f.Accounts, nil
}

func (f *FakeAccountRepository) FindByID(id string) (*domain.Account, error) {
	for i := range f.Accounts {
		if f.Accounts[i].ID == id {
			found := f.Accounts[i]
			return &found, nil
		}
	}
	return nil, nil
}

func (f *FakeAccountRepository) FindByFingerprint(fp string) (*domain.Account, error) {
	for i := range f.Accounts {
		if f.Accounts[i].PublicKeyFingerprint == fp {
			found := f.Accounts[i]
			return &found, nil
		}
	}
	return nil, nil
}

func (f *FakeAccountRepository) Save(account domain.Account) error {
	for _, a := range f.Accounts {
		if a.ID == account.ID {
			return fmt.Errorf("account %q already exists", account.ID)
		}
	}
	f.Accounts = append(f.Accounts, account)
	return nil
}

func (f *FakeAccountRepository) Update(account domain.Account) error {
	for i, a := range f.Accounts {
		if a.ID == account.ID {
			f.Accounts[i] = account
			return nil
		}
	}
	return fmt.Errorf("account %q not found", account.ID)
}

func (f *FakeAccountRepository) Delete(id string) error {
	for i, a := range f.Accounts {
		if a.ID == id {
			f.Accounts = append(f.Accounts[:i], f.Accounts[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("account %q not found", id)
}
