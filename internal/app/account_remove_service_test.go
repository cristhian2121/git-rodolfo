package app_test

import (
	"errors"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func TestAccountRemoveService_RemoveProfile(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/k"})
	svc := app.NewAccountRemoveService(repo, fakes.NewFakeSSHClient())

	removed, err := svc.RemoveProfile("lean-tech")
	if err != nil {
		t.Fatalf("RemoveProfile: %v", err)
	}
	if removed.DisplayName != "Lean Tech" {
		t.Fatalf("unexpected removed account: %+v", removed)
	}
	if accounts, _ := repo.List(); len(accounts) != 0 {
		t.Fatalf("expected account to be gone, got %v", accounts)
	}
}

func TestAccountRemoveService_RemoveProfile_NotFound(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	svc := app.NewAccountRemoveService(repo, fakes.NewFakeSSHClient())

	_, err := svc.RemoveProfile("ghost")
	if !errors.Is(err, app.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestAccountRemoveService_DeleteKeyFiles(t *testing.T) {
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/home/u/.ssh/id_ed25519"] = fakes.FakeKey{Valid: true}
	svc := app.NewAccountRemoveService(fakes.NewFakeAccountRepository(), ssh)

	err := svc.DeleteKeyFiles(domain.Account{PrivateKeyPath: "/home/u/.ssh/id_ed25519"})
	if err != nil {
		t.Fatalf("DeleteKeyFiles: %v", err)
	}
	if len(ssh.DeletedKeys) != 1 || ssh.DeletedKeys[0] != "/home/u/.ssh/id_ed25519" {
		t.Fatalf("key was not deleted: %+v", ssh.DeletedKeys)
	}
}

func TestAccountRemoveService_DeleteKeyFiles_NoKeyIsNoop(t *testing.T) {
	svc := app.NewAccountRemoveService(fakes.NewFakeAccountRepository(), fakes.NewFakeSSHClient())
	if err := svc.DeleteKeyFiles(domain.Account{}); err != nil {
		t.Fatalf("expected no error for an account without a key, got %v", err)
	}
}

func TestAccountRemoveService_KeyStillUsedByAnotherAccount(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(
		domain.Account{ID: "personal", DisplayName: "Personal", PublicKeyFingerprint: "SHA256:shared"},
	)
	svc := app.NewAccountRemoveService(repo, fakes.NewFakeSSHClient())

	used, err := svc.KeyStillUsedByAnotherAccount(domain.Account{ID: "lean-tech", PublicKeyFingerprint: "SHA256:shared"})
	if err != nil {
		t.Fatalf("KeyStillUsedByAnotherAccount: %v", err)
	}
	if !used {
		t.Fatal("expected the key to be reported as still in use by another account")
	}

	unused, err := svc.KeyStillUsedByAnotherAccount(domain.Account{ID: "lean-tech", PublicKeyFingerprint: "SHA256:unique"})
	if err != nil {
		t.Fatalf("KeyStillUsedByAnotherAccount: %v", err)
	}
	if unused {
		t.Fatal("expected the key to be reported as not shared")
	}

	// An account checking against its own still-registered fingerprint
	// (i.e. before it has actually been deleted) must not count as "used
	// by another account".
	self, err := svc.KeyStillUsedByAnotherAccount(domain.Account{ID: "personal", PublicKeyFingerprint: "SHA256:shared"})
	if err != nil {
		t.Fatalf("KeyStillUsedByAnotherAccount: %v", err)
	}
	if self {
		t.Fatal("an account's own fingerprint must not count as used by another account")
	}
}
