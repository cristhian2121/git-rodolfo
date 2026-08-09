package cli_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

// fullDeps builds a Deps with every service wired to fakes — used by
// edit/remove tests, which exercise more of the surface than a single
// command's own helper covers.
func fullDeps(t *testing.T, stdin *os.File, repo *fakes.FakeAccountRepository, ssh *fakes.FakeSSHClient) (cli.Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	agent := fakes.NewFakeSSHAgentAdapter()
	return cli.Deps{
		Accounts:   app.NewAccountService(repo),
		AccountAdd: app.NewAccountAddService(repo, ssh, agent, fixedNow),
		Edit:       app.NewAccountEditService(repo, ssh, fixedNow),
		Remove:     app.NewAccountRemoveService(repo, ssh),
		Auth:       app.NewAuthenticationService(ssh, repo, fixedNow),
		SSH:        ssh,
		Agent:      agent,
		Provider:   fakes.NewFakeProviderClient(),
		SSHDir:     t.TempDir(),
		Stdout:     &stdout,
		Stderr:     &stderr,
		Stdin:      stdin,
		Now:        fixedNow,
	}, &stdout, &stderr
}

func TestRun_AccountEditNonInteractive_ChangeEmail(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "old@leantech.com",
	})
	deps, stdout, stderr := fullDeps(t, nil, repo, fakes.NewFakeSSHClient())

	code := cli.Run([]string{
		"account", "edit", "lean-tech", "--non-interactive",
		"--commit-email", "new@leantech.com",
	}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `Account "Lean Tech" updated.`) {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "old@leantech.com\n→ new@leantech.com") {
		t.Fatalf("missing diff line: %s", stdout.String())
	}
	saved, _ := repo.FindByID("lean-tech")
	if saved.GitEmail != "new@leantech.com" {
		t.Fatalf("email not persisted: %+v", saved)
	}
}

func TestRun_AccountEditNonInteractive_RepoWarning(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "old@leantech.com"})
	deps, stdout, _ := fullDeps(t, nil, repo, fakes.NewFakeSSHClient())

	code := cli.Run([]string{
		"account", "edit", "lean-tech", "--non-interactive",
		"--commit-email", "new@leantech.com",
	}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(stdout.String(), `Run "git rodolfo use lean-tech" inside each affected repository`) {
		t.Fatalf("missing repo-affected warning: %s", stdout.String())
	}
}

func TestRun_AccountEditNonInteractive_NoFieldsGiven(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech"})
	deps, _, stderr := fullDeps(t, nil, repo, fakes.NewFakeSSHClient())

	code := cli.Run([]string{"account", "edit", "lean-tech", "--non-interactive"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "specify at least one field to change") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRun_AccountEditNonInteractive_ChangeKeyAndDeleteOld(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/old/id_ed25519",
	})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/old/id_ed25519"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:old"}
	ssh.Keys["/new/id_ed25519"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:new"}
	deps, stdout, stderr := fullDeps(t, nil, repo, ssh)

	code := cli.Run([]string{
		"account", "edit", "lean-tech", "--non-interactive",
		"--key", "/new/id_ed25519",
		"--delete-old-key",
	}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "✓ Previous key files deleted.") {
		t.Fatalf("missing deletion confirmation: %s", stdout.String())
	}
	if len(ssh.DeletedKeys) != 1 || ssh.DeletedKeys[0] != "/old/id_ed25519" {
		t.Fatalf("old key was not deleted: %+v", ssh.DeletedKeys)
	}
	saved, _ := repo.FindByID("lean-tech")
	if saved.PrivateKeyPath != "/new/id_ed25519" {
		t.Fatalf("key not updated: %+v", saved)
	}
}

func TestRun_AccountEditNonInteractive_DuplicateKeyRejected(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(
		domain.Account{ID: "personal", DisplayName: "Personal", PublicKeyFingerprint: "SHA256:shared"},
		domain.Account{ID: "lean-tech", DisplayName: "Lean Tech"},
	)
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/shared/id_ed25519"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:shared"}
	deps, _, stderr := fullDeps(t, nil, repo, ssh)

	code := cli.Run([]string{
		"account", "edit", "lean-tech", "--non-interactive",
		"--key", "/shared/id_ed25519",
	}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `is already used by the account "Personal"`) {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

// TestRun_AccountEditInteractive_ConfirmAndApply drives the wizard through
// a pipe (never a TTY, so Select/Confirm/Text all use their deterministic
// fallbacks).
func TestRun_AccountEditInteractive_ConfirmAndApply(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", GitName: "Cristhian Delgado",
		GitEmail: "old@leantech.com", ProviderUsername: "old-username",
	})
	deps, stdout, stderr := fullDeps(t, r, repo, fakes.NewFakeSSHClient())

	go func() {
		defer w.Close()
		w.WriteString("\n")                 // keep display name
		w.WriteString("\n")                 // keep commit name
		w.WriteString("new@leantech.com\n") // change commit email
		w.WriteString("\n")                 // keep username
		w.WriteString("n\n")                // don't change the SSH key
		w.WriteString("y\n")                // confirm the update
	}()

	code := cli.Run([]string{"account", "edit", "lean-tech"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "✓ Account updated.") {
		t.Fatalf("missing confirmation: %s", stdout.String())
	}
	saved, _ := repo.FindByID("lean-tech")
	if saved.GitEmail != "new@leantech.com" {
		t.Fatalf("email not updated: %+v", saved)
	}
}

func TestRun_AccountEditInteractive_DeclineAborts(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "old@leantech.com"})
	deps, stdout, _ := fullDeps(t, r, repo, fakes.NewFakeSSHClient())

	go func() {
		defer w.Close()
		w.WriteString("\n")
		w.WriteString("\n")
		w.WriteString("new@leantech.com\n")
		w.WriteString("\n")
		w.WriteString("n\n")
		w.WriteString("n\n") // decline the confirmation
	}()

	code := cli.Run([]string{"account", "edit", "lean-tech"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1, stdout = %s", code, stdout.String())
	}
	saved, _ := repo.FindByID("lean-tech")
	if saved.GitEmail != "old@leantech.com" {
		t.Fatalf("declining must not persist the change: %+v", saved)
	}
}
