package app_test

import (
	"errors"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

// TestSSHCommandMechanism_Apply_RejectsUnsafeKeyPath is the defense-in-depth
// regression test at the point sshCommandFor actually embeds the path in a
// single-quoted shell string: even if some future caller skipped the
// application-layer validation, Apply itself must still refuse.
func TestSSHCommandMechanism_Apply_RejectsUnsafeKeyPath(t *testing.T) {
	git := fakes.NewFakeGitClient()
	m := app.NewSSHCommandMechanism()
	account := domain.Account{
		ID: "lean-tech", GitName: "N", GitEmail: "e@example.com",
		PrivateKeyPath: "/home/u/.ssh/id_ed25519'; touch /tmp/PWNED; echo '",
	}

	err := m.Apply(git, account)
	if !errors.Is(err, app.ErrUnsafeKeyPath) {
		t.Fatalf("expected ErrUnsafeKeyPath, got %v", err)
	}
	if _, ok := git.LocalConfig["core.sshCommand"]; ok {
		t.Fatal("core.sshCommand must not be set when the key path is rejected")
	}
}

func TestSSHCommandMechanism_Apply(t *testing.T) {
	git := fakes.NewFakeGitClient()
	m := app.NewSSHCommandMechanism()
	account := domain.Account{ID: "lean-tech", GitName: "Cristhian Delgado", GitEmail: "cristhian@leantech.com", PrivateKeyPath: "/home/u/.ssh/id_ed25519_leantech"}

	if err := m.Apply(git, account); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if git.LocalConfig["user.name"] != "Cristhian Delgado" {
		t.Fatalf("user.name = %q", git.LocalConfig["user.name"])
	}
	if git.LocalConfig["user.email"] != "cristhian@leantech.com" {
		t.Fatalf("user.email = %q", git.LocalConfig["user.email"])
	}
	if git.LocalConfig["rodolfo.account"] != "lean-tech" {
		t.Fatalf("rodolfo.account = %q", git.LocalConfig["rodolfo.account"])
	}
	want := "ssh -i '/home/u/.ssh/id_ed25519_leantech' -o IdentitiesOnly=yes"
	if git.LocalConfig["core.sshCommand"] != want {
		t.Fatalf("core.sshCommand = %q, want %q", git.LocalConfig["core.sshCommand"], want)
	}
}

func TestSSHCommandMechanism_Apply_NoKeySkipsSSHCommand(t *testing.T) {
	git := fakes.NewFakeGitClient()
	m := app.NewSSHCommandMechanism()
	account := domain.Account{ID: "lean-tech", GitName: "N", GitEmail: "e@example.com"}

	if err := m.Apply(git, account); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, ok := git.LocalConfig["core.sshCommand"]; ok {
		t.Fatal("expected no core.sshCommand set when the account has no key")
	}
}

func TestSSHCommandMechanism_Clear(t *testing.T) {
	git := fakes.NewFakeGitClient()
	m := app.NewSSHCommandMechanism()
	account := domain.Account{ID: "lean-tech", GitName: "N", GitEmail: "e@example.com", PrivateKeyPath: "/k"}
	if err := m.Apply(git, account); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if err := m.Clear(git); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if len(git.LocalConfig) != 0 {
		t.Fatalf("expected no local config left, got %v", git.LocalConfig)
	}

	// RF-20: clearing an already-clear repo must not error.
	if err := m.Clear(git); err != nil {
		t.Fatalf("Clear on an already-clear repo: %v", err)
	}
}

func TestSSHCommandMechanism_Verify_OK(t *testing.T) {
	git := fakes.NewFakeGitClient()
	m := app.NewSSHCommandMechanism()
	account := domain.Account{ID: "lean-tech", GitName: "N", GitEmail: "e@example.com", PrivateKeyPath: "/k"}
	if err := m.Apply(git, account); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	result, err := m.Verify(git, account)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !result.OK {
		t.Fatalf("expected OK, got issues: %v", result.Issues)
	}
}

func TestSSHCommandMechanism_Verify_DetectsDrift(t *testing.T) {
	git := fakes.NewFakeGitClient()
	m := app.NewSSHCommandMechanism()
	account := domain.Account{ID: "lean-tech", GitName: "N", GitEmail: "cristhian@leantech.com", PrivateKeyPath: "/k"}
	if err := m.Apply(git, account); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Simulate the repo's email drifting away from the assigned account.
	git.LocalConfig["user.email"] = "cristhian@gmail.com"

	result, err := m.Verify(git, account)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.OK {
		t.Fatal("expected drift to be detected")
	}
	if len(result.Issues) != 1 {
		t.Fatalf("expected exactly one issue, got %v", result.Issues)
	}
}

func TestSSHCommandMechanism_Verify_UnmanagedRepoIsNotOK(t *testing.T) {
	git := fakes.NewFakeGitClient() // nothing configured at all
	m := app.NewSSHCommandMechanism()
	account := domain.Account{ID: "lean-tech", GitName: "N", GitEmail: "e@example.com", PrivateKeyPath: "/k"}

	result, err := m.Verify(git, account)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if result.OK {
		t.Fatal("an unconfigured repo must not verify as OK against a specific account")
	}
}
