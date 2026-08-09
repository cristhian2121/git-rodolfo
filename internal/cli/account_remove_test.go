package cli_test

import (
	"os"
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"

	"github.com/lean-tech/git-rodolfo/internal/cli"
)

func TestRun_AccountRemoveNonInteractive_RequiresYes(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech"})
	deps, _, stderr := fullDeps(t, nil, repo, fakes.NewFakeSSHClient())

	code := cli.Run([]string{"account", "remove", "lean-tech", "--non-interactive"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires --yes") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if accounts, _ := repo.List(); len(accounts) != 1 {
		t.Fatal("account must not be removed without --yes")
	}
}

func TestRun_AccountRemoveNonInteractive_KeepsKeyByDefault(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/k"})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/k"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:x"}
	deps, stdout, stderr := fullDeps(t, nil, repo, ssh)

	code := cli.Run([]string{"account", "remove", "lean-tech", "--non-interactive", "--yes"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "✓ Account removed.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if accounts, _ := repo.List(); len(accounts) != 0 {
		t.Fatal("account should be removed")
	}
	if len(ssh.DeletedKeys) != 0 {
		t.Fatal("--yes alone must never delete the key (RF-08)")
	}
}

func TestRun_AccountRemoveNonInteractive_DeleteKeyRequiresExplicitFlag(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/k"})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/k"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:x"}
	deps, stdout, stderr := fullDeps(t, nil, repo, ssh)

	code := cli.Run([]string{"account", "remove", "lean-tech", "--non-interactive", "--yes", "--delete-key"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "✓ Key files deleted.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if len(ssh.DeletedKeys) != 1 || ssh.DeletedKeys[0] != "/k" {
		t.Fatalf("expected key to be deleted, got %+v", ssh.DeletedKeys)
	}
}

func TestRun_AccountRemoveNonInteractive_KeyStillSharedIsNeverOffered(t *testing.T) {
	// Defensive case: shouldn't normally happen (RF-07 prevents it going
	// forward), but if a key's fingerprint is somehow still claimed by
	// another account, removal must not delete it even with --delete-key.
	repo := fakes.NewFakeAccountRepository(
		domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/k", PublicKeyFingerprint: "SHA256:shared"},
		domain.Account{ID: "other", DisplayName: "Other", PublicKeyFingerprint: "SHA256:shared"},
	)
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/k"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:shared"}
	deps, stdout, stderr := fullDeps(t, nil, repo, ssh)

	code := cli.Run([]string{"account", "remove", "lean-tech", "--non-interactive", "--yes", "--delete-key"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "still used by another registered account") {
		t.Fatalf("missing shared-key notice: %s", stdout.String())
	}
	if len(ssh.DeletedKeys) != 0 {
		t.Fatal("a key still claimed by another account must never be deleted")
	}
}

func TestRun_AccountRemoveNonInteractive_RequiresAccountID(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech"})
	deps, _, stderr := fullDeps(t, nil, repo, fakes.NewFakeSSHClient())

	code := cli.Run([]string{"account", "remove", "--non-interactive", "--yes"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires an account id") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

// TestRun_AccountRemoveInteractive_DeclineProfileKeepsEverything covers the
// step-1 decline path: nothing happens, exit is clean (0), matching a
// user backing out of a destructive action being a normal outcome.
func TestRun_AccountRemoveInteractive_DeclineProfileKeepsEverything(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/k"})
	deps, stdout, _ := fullDeps(t, r, repo, fakes.NewFakeSSHClient())

	go func() {
		defer w.Close()
		w.WriteString("n\n") // decline step 1
	}()

	code := cli.Run([]string{"account", "remove", "lean-tech"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, want 0, stdout = %s", code, stdout.String())
	}
	if !strings.Contains(stdout.String(), "Aborted. No changes were made.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if accounts, _ := repo.List(); len(accounts) != 1 {
		t.Fatal("declining must not remove the account")
	}
}

// TestRun_AccountRemoveInteractive_ConfirmBothSteps drives both
// confirmations through the fallback (piped, non-TTY) prompts.
func TestRun_AccountRemoveInteractive_ConfirmBothSteps(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/k"})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/k"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:x"}
	deps, stdout, stderr := fullDeps(t, r, repo, ssh)

	go func() {
		defer w.Close()
		w.WriteString("y\n") // confirm profile removal
		w.WriteString("y\n") // confirm key deletion
	}()

	code := cli.Run([]string{"account", "remove", "lean-tech"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "✓ Account removed.") || !strings.Contains(stdout.String(), "✓ Key files deleted.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if len(ssh.DeletedKeys) != 1 {
		t.Fatal("expected the key to be deleted after confirming both steps")
	}
}

// TestRun_AccountRemoveInteractive_SelectThenConfirmBothSteps drives the
// no-id path (select from a list) followed by both confirmations through a
// single pipe. This is a regression test: selection and confirmation used
// to run on separate prompt.Session instances sharing one Stdin, and the
// first session's internal buffering could silently swallow input meant
// for the second prompt.
func TestRun_AccountRemoveInteractive_SelectThenConfirmBothSteps(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	repo := fakes.NewFakeAccountRepository(
		domain.Account{ID: "personal", DisplayName: "Personal"},
		domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/k"},
	)
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/k"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:x"}
	deps, stdout, stderr := fullDeps(t, r, repo, ssh)

	go func() {
		defer w.Close()
		w.WriteString("2\n") // select "Lean Tech" from the numbered fallback list
		w.WriteString("y\n") // confirm profile removal
		w.WriteString("y\n") // confirm key deletion
	}()

	code := cli.Run([]string{"account", "remove"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "✓ Account removed.") || !strings.Contains(stdout.String(), "✓ Key files deleted.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if accounts, _ := repo.List(); len(accounts) != 1 || accounts[0].ID != "personal" {
		t.Fatalf("expected only \"personal\" to remain, got %+v", accounts)
	}
}

func TestRun_AccountRemoveInteractive_ConfirmProfileDeclineKey(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/k"})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/k"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:x"}
	deps, stdout, _ := fullDeps(t, r, repo, ssh)

	go func() {
		defer w.Close()
		w.WriteString("y\n") // confirm profile removal
		w.WriteString("n\n") // decline key deletion
	}()

	code := cli.Run([]string{"account", "remove", "lean-tech"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stdout = %s", code, stdout.String())
	}
	if len(ssh.DeletedKeys) != 0 {
		t.Fatal("declining step 2 must keep the key files")
	}
	if accounts, _ := repo.List(); len(accounts) != 0 {
		t.Fatal("the profile removal from step 1 must still have taken effect")
	}
}
