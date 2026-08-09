package app_test

import (
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func TestAuthenticationService_Success(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech",
		ProviderUsername: "cristhiandelgado-work",
		Hostname:         "github.com",
		PrivateKeyPath:   "/home/u/.ssh/id_ed25519_leantech",
	})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/home/u/.ssh/id_ed25519_leantech"] = fakes.FakeKey{
		AuthResult: domain.AuthResult{Success: true, Username: "cristhiandelgado-work", RawOutput: "Hi cristhiandelgado-work!"},
	}
	svc := app.NewAuthenticationService(ssh, repo, fixedClock)

	outcome, err := svc.Verify(mustGet(repo, "lean-tech"))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if outcome.UsernameMismatch {
		t.Fatal("expected no mismatch")
	}
	if outcome.Account.AuthenticationStatus != domain.AuthenticationStatusVerified {
		t.Fatalf("status = %q, want verified", outcome.Account.AuthenticationStatus)
	}
	if outcome.Account.LastVerifiedAt == nil || !outcome.Account.LastVerifiedAt.Equal(fixedClock()) {
		t.Fatalf("LastVerifiedAt not set to clock time: %+v", outcome.Account.LastVerifiedAt)
	}

	persisted, _ := repo.FindByID("lean-tech")
	if persisted.AuthenticationStatus != domain.AuthenticationStatusVerified {
		t.Fatalf("update not persisted: %+v", persisted)
	}
}

func TestAuthenticationService_Failure(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech",
		ProviderUsername: "cristhiandelgado-work",
		Hostname:         "github.com",
		PrivateKeyPath:   "/home/u/.ssh/id_ed25519_leantech",
	})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/home/u/.ssh/id_ed25519_leantech"] = fakes.FakeKey{
		AuthResult: domain.AuthResult{Success: false, RawOutput: "Permission denied (publickey)."},
	}
	svc := app.NewAuthenticationService(ssh, repo, fixedClock)

	outcome, err := svc.Verify(mustGet(repo, "lean-tech"))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if outcome.Account.AuthenticationStatus != domain.AuthenticationStatusFailed {
		t.Fatalf("status = %q, want failed", outcome.Account.AuthenticationStatus)
	}
}

// TestAuthenticationService_UsernameMismatch is §21 "the key authenticates
// as another user": SSH succeeds, but for a different GitHub account than
// this one was registered under.
func TestAuthenticationService_UsernameMismatch(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech",
		ProviderUsername: "cristhiandelgado-work",
		Hostname:         "github.com",
		PrivateKeyPath:   "/home/u/.ssh/id_ed25519_leantech",
	})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/home/u/.ssh/id_ed25519_leantech"] = fakes.FakeKey{
		AuthResult: domain.AuthResult{Success: true, Username: "cristhiandelgado", RawOutput: "Hi cristhiandelgado!"},
	}
	svc := app.NewAuthenticationService(ssh, repo, fixedClock)

	outcome, err := svc.Verify(mustGet(repo, "lean-tech"))
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !outcome.UsernameMismatch {
		t.Fatal("expected a username mismatch")
	}
	if outcome.Account.AuthenticationStatus != domain.AuthenticationStatusFailed {
		t.Fatalf("a mismatch must not count as verified, got %q", outcome.Account.AuthenticationStatus)
	}
}

func TestAuthenticationService_NoKeyConfigured(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	svc := app.NewAuthenticationService(fakes.NewFakeSSHClient(), repo, fixedClock)

	_, err := svc.Verify(domain.Account{DisplayName: "No Key"})
	if err == nil {
		t.Fatal("expected an error when no key is configured")
	}
}

func mustGet(repo *fakes.FakeAccountRepository, id string) domain.Account {
	a, _ := repo.FindByID(id)
	return *a
}
