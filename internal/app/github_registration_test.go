package app_test

import (
	"errors"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func TestRegisterPublicKeyIfMatchingSession_Match(t *testing.T) {
	provider := fakes.NewFakeProviderClient()
	provider.CLIUsername = "cristhiandelgado-work"
	account := domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", ProviderUsername: "cristhiandelgado-work"}

	if err := app.RegisterPublicKeyIfMatchingSession(provider, account, "ssh-ed25519 AAAA..."); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if len(provider.RegisteredKeys) != 1 || provider.RegisteredKeys[0].AccountID != account.ID {
		t.Fatalf("key was not registered: %+v", provider.RegisteredKeys)
	}
}

func TestRegisterPublicKeyIfMatchingSession_Mismatch(t *testing.T) {
	provider := fakes.NewFakeProviderClient()
	provider.CLIUsername = "cristhiandelgado"
	account := domain.Account{DisplayName: "Lean Tech", ProviderUsername: "cristhiandelgado-work"}

	err := app.RegisterPublicKeyIfMatchingSession(provider, account, "ssh-ed25519 AAAA...")

	var mismatch *app.GitHubCLIMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected GitHubCLIMismatchError, got %v", err)
	}
	if mismatch.ActiveUsername != "cristhiandelgado" || mismatch.AccountUsername != "cristhiandelgado-work" {
		t.Fatalf("unexpected mismatch details: %+v", mismatch)
	}
	if len(provider.RegisteredKeys) != 0 {
		t.Fatal("key must not be registered on a mismatch")
	}
}
