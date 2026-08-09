package app_test

import (
	"errors"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func TestAccountEditService_ChangeEmailOnly(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech",
		GitName: "Cristhian Delgado", GitEmail: "old@leantech.com",
		PrivateKeyPath: "/k",
	})
	svc := app.NewAccountEditService(repo, fakes.NewFakeSSHClient(), fixedClock)

	result, err := svc.Edit(app.AccountEditInput{ID: "lean-tech", GitEmail: "new@leantech.com"})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if result.After.GitEmail != "new@leantech.com" {
		t.Fatalf("email not updated: %+v", result.After)
	}
	if result.Before.GitEmail != "old@leantech.com" {
		t.Fatalf("Before snapshot wrong: %+v", result.Before)
	}
	if !result.RepoConfigAffected {
		t.Fatal("expected RepoConfigAffected: commit email is written to .git/config")
	}
	if result.OldKeyPath != "" {
		t.Fatalf("key wasn't touched, OldKeyPath should be empty, got %q", result.OldKeyPath)
	}

	persisted, _ := repo.FindByID("lean-tech")
	if persisted.GitEmail != "new@leantech.com" {
		t.Fatal("update not persisted")
	}
}

func TestAccountEditService_DisplayNameOnly_DoesNotAffectRepoConfig(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech"})
	svc := app.NewAccountEditService(repo, fakes.NewFakeSSHClient(), fixedClock)

	result, err := svc.Edit(app.AccountEditInput{ID: "lean-tech", DisplayName: "Lean Tech Renamed"})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if result.RepoConfigAffected {
		t.Fatal("a display name change alone must not affect .git/config")
	}
}

func TestAccountEditService_ChangeKey(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech",
		PrivateKeyPath: "/home/u/.ssh/id_old", PublicKeyFingerprint: "SHA256:old",
		AuthenticationStatus: domain.AuthenticationStatusVerified,
	})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/home/u/.ssh/id_new"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:new"}
	svc := app.NewAccountEditService(repo, ssh, fixedClock)

	result, err := svc.Edit(app.AccountEditInput{ID: "lean-tech", NewKeyPath: "/home/u/.ssh/id_new"})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if result.After.PrivateKeyPath != "/home/u/.ssh/id_new" || result.After.PublicKeyFingerprint != "SHA256:new" {
		t.Fatalf("key not updated: %+v", result.After)
	}
	if result.After.AuthenticationStatus != domain.AuthenticationStatusUnverified {
		t.Fatalf("expected status to reset to unverified after a key change, got %q", result.After.AuthenticationStatus)
	}
	if result.OldKeyPath != "/home/u/.ssh/id_old" {
		t.Fatalf("OldKeyPath = %q, want the previous key path", result.OldKeyPath)
	}
	if !result.RepoConfigAffected {
		t.Fatal("a key change must affect .git/config (core.sshCommand)")
	}
}

func TestAccountEditService_ChangeKey_DuplicateRejected(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(
		domain.Account{ID: "personal", DisplayName: "Personal", PublicKeyFingerprint: "SHA256:shared"},
		domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/old"},
	)
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/home/u/.ssh/id_shared"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:shared"}
	svc := app.NewAccountEditService(repo, ssh, fixedClock)

	_, err := svc.Edit(app.AccountEditInput{ID: "lean-tech", NewKeyPath: "/home/u/.ssh/id_shared"})

	var dup *app.KeyAlreadyInUseError
	if !errors.As(err, &dup) {
		t.Fatalf("expected KeyAlreadyInUseError, got %v", err)
	}
}

// TestAccountEditService_RejectsKeyPathWithSingleQuote is the same
// shell-injection regression as AccountAddService's, for the edit path.
func TestAccountEditService_RejectsKeyPathWithSingleQuote(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/old"})
	ssh := fakes.NewFakeSSHClient()
	maliciousPath := "/home/u/.ssh/id_ed25519'; touch /tmp/PWNED; echo '"
	ssh.Keys[maliciousPath] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:evil"}
	svc := app.NewAccountEditService(repo, ssh, fixedClock)

	_, err := svc.Preview(app.AccountEditInput{ID: "lean-tech", NewKeyPath: maliciousPath})
	if !errors.Is(err, app.ErrUnsafeKeyPath) {
		t.Fatalf("expected ErrUnsafeKeyPath, got %v", err)
	}
}

func TestAccountEditService_PreviewDoesNotCommit(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "old@leantech.com"})
	svc := app.NewAccountEditService(repo, fakes.NewFakeSSHClient(), fixedClock)

	result, err := svc.Preview(app.AccountEditInput{ID: "lean-tech", GitEmail: "new@leantech.com"})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if result.After.GitEmail != "new@leantech.com" {
		t.Fatalf("preview result wrong: %+v", result.After)
	}

	persisted, _ := repo.FindByID("lean-tech")
	if persisted.GitEmail != "old@leantech.com" {
		t.Fatalf("Preview must not persist changes, but repo shows: %+v", persisted)
	}

	if err := svc.Commit(result.After); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	persisted, _ = repo.FindByID("lean-tech")
	if persisted.GitEmail != "new@leantech.com" {
		t.Fatal("Commit did not persist the previewed change")
	}
}

func TestAccountEditService_NotFound(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	svc := app.NewAccountEditService(repo, fakes.NewFakeSSHClient(), fixedClock)

	_, err := svc.Edit(app.AccountEditInput{ID: "ghost", GitEmail: "x@example.com"})
	if !errors.Is(err, app.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}
