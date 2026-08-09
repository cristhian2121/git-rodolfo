package app_test

import (
	"errors"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func TestAccountService_List(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(
		domain.Account{ID: "personal", DisplayName: "Personal"},
		domain.Account{ID: "lean-tech", DisplayName: "Lean Tech"},
	)
	svc := app.NewAccountService(repo)

	accounts, err := svc.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(accounts))
	}
}

func TestAccountService_Get(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(
		domain.Account{ID: "lean-tech", DisplayName: "Lean Tech"},
	)
	svc := app.NewAccountService(repo)

	acc, err := svc.Get("lean-tech")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if acc.DisplayName != "Lean Tech" {
		t.Fatalf("unexpected account: %+v", acc)
	}

	_, err = svc.Get("missing")
	if !errors.Is(err, app.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}
