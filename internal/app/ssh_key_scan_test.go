package app_test

import (
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func TestScanAvailableKeys(t *testing.T) {
	ssh := fakes.NewFakeSSHClient()
	dir := "/home/user/.ssh"
	ssh.ScanResults = map[string][]domain.SSHKeyCandidate{
		dir: {
			{PrivateKeyPath: dir + "/id_free", PublicKeyPath: dir + "/id_free.pub", Fingerprint: "SHA256:free"},
			{PrivateKeyPath: dir + "/id_taken", PublicKeyPath: dir + "/id_taken.pub", Fingerprint: "SHA256:taken"},
		},
	}

	accounts := fakes.NewFakeAccountRepository(domain.Account{
		ID:                   "lean-tech",
		DisplayName:          "Lean Tech",
		PublicKeyFingerprint: "SHA256:taken",
	})

	got, err := app.ScanAvailableKeys(ssh, accounts, dir)
	if err != nil {
		t.Fatalf("ScanAvailableKeys: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d options, want 2", len(got))
	}
	if got[0].PrivateKeyPath != dir+"/id_free" || got[0].InUseBy != "" {
		t.Fatalf("free key option = %+v", got[0])
	}
	if got[1].PrivateKeyPath != dir+"/id_taken" || got[1].InUseBy != "Lean Tech" {
		t.Fatalf("taken key option = %+v", got[1])
	}
}

func TestScanAvailableKeys_NoCandidates(t *testing.T) {
	ssh := fakes.NewFakeSSHClient()
	accounts := fakes.NewFakeAccountRepository()

	got, err := app.ScanAvailableKeys(ssh, accounts, "/home/user/.ssh")
	if err != nil {
		t.Fatalf("ScanAvailableKeys: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d options, want 0", len(got))
	}
}
