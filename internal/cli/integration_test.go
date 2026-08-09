package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

// These are the Sprint 4 "Definición de hecho" integration tests: real
// local Git repositories, real `git` binary, no network and no GitHub
// (RNF-09) — only the two-real-accounts SSH validation from §12.2's
// validation criteria needs a real GitHub setup, which is out of reach for
// an automated test and is covered by the Sprint 6 manual e2e suite
// instead.

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
}

func realDeps(t *testing.T, repoDir string, accounts *fakes.FakeAccountRepository) (cli.Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	mechanism := app.NewSSHCommandMechanism()
	newGitClient := func(dir string) app.GitClient { return infra.NewGitClient(dir) }
	return cli.Deps{
		Accounts:     app.NewAccountService(accounts),
		AccountRepo:  accounts,
		Mechanism:    mechanism,
		NewGitClient: newGitClient,
		Getwd:        func() (string, error) { return repoDir, nil },
		Stdout:       &stdout,
		Stderr:       &stderr,
		Now:          fixedNow,
	}, &stdout, &stderr
}

// TestIntegration_Use_GitInitNoRemote is CU-09 / RF-19: `use` must
// configure identity on a repository created with `git init`, with no
// remote at all, without error.
func TestIntegration_Use_GitInitNoRemote(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "--quiet", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	accounts := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", GitName: "Cristhian Delgado",
		GitEmail: "cristhian@leantech.com", PrivateKeyPath: "/home/u/.ssh/id_ed25519_leantech",
	})
	deps, stdout, stderr := realDeps(t, dir, accounts)

	code := cli.Run([]string{"use", "lean-tech", "--non-interactive"}, deps)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Remote: none") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}

	out, err := exec.Command("git", "-C", dir, "config", "--local", "--list").CombinedOutput()
	if err != nil {
		t.Fatalf("git config --local --list: %v: %s", err, out)
	}
	list := string(out)
	for _, want := range []string{
		"user.name=Cristhian Delgado",
		"user.email=cristhian@leantech.com",
		"rodolfo.account=lean-tech",
		"core.sshcommand=ssh -i '/home/u/.ssh/id_ed25519_leantech' -o IdentitiesOnly=yes",
	} {
		if !strings.Contains(list, want) {
			t.Fatalf("git config --local --list missing %q:\n%s", want, list)
		}
	}
}

// TestIntegration_UseThenClear_LeavesNoTrace is RF-20's own validation
// criterion, verified exactly as §12.2 specifies: `git config --local
// --list` must show nothing Git Rodolfo wrote, after `use --clear`.
func TestIntegration_UseThenClear_LeavesNoTrace(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "--quiet", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	accounts := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", GitName: "Cristhian Delgado",
		GitEmail: "cristhian@leantech.com", PrivateKeyPath: "/home/u/.ssh/id_ed25519_leantech",
	})
	deps, _, stderr := realDeps(t, dir, accounts)

	if code := cli.Run([]string{"use", "lean-tech", "--non-interactive"}, deps); code != 0 {
		t.Fatalf("use: exit code = %d, stderr = %s", code, stderr.String())
	}

	deps2, stdout2, stderr2 := realDeps(t, dir, accounts)
	if code := cli.Run([]string{"use", "--clear"}, deps2); code != 0 {
		t.Fatalf("use --clear: exit code = %d, stderr = %s", code, stderr2.String())
	}
	if !strings.Contains(stdout2.String(), "Repository identity cleared.") {
		t.Fatalf("unexpected output: %s", stdout2.String())
	}

	out, err := exec.Command("git", "-C", dir, "config", "--local", "--list").CombinedOutput()
	if err != nil {
		// An empty --local config can make git exit non-zero with no
		// output at all on some versions; either way, output must be empty.
		if len(out) != 0 {
			t.Fatalf("git config --local --list: %v: %s", err, out)
		}
	}
	for _, mustNotContain := range []string{"user.name", "user.email", "rodolfo.account", "core.sshcommand"} {
		if strings.Contains(string(out), mustNotContain) {
			t.Fatalf("expected %q to be gone from local config, got:\n%s", mustNotContain, out)
		}
	}
}

// TestIntegration_Current_DetectsDriftOnRealRepo exercises "current"
// against a real repo where the local email was hand-edited after `use`,
// simulating exactly the drift scenario CU-10 describes.
func TestIntegration_Current_DetectsDriftOnRealRepo(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "--quiet", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	accounts := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", GitName: "Cristhian Delgado",
		GitEmail: "cristhian@leantech.com",
	})
	deps, _, stderr := realDeps(t, dir, accounts)
	if code := cli.Run([]string{"use", "lean-tech", "--non-interactive"}, deps); code != 0 {
		t.Fatalf("use: exit code = %d, stderr = %s", code, stderr.String())
	}

	if out, err := exec.Command("git", "-C", dir, "config", "--local", "user.email", "cristhian@gmail.com").CombinedOutput(); err != nil {
		t.Fatalf("simulate drift: %v: %s", err, out)
	}

	deps2, stdout2, stderr2 := realDeps(t, dir, accounts)
	code := cli.Run([]string{"current"}, deps2)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr2.String())
	}
	if !strings.Contains(stdout2.String(), "Warning:") {
		t.Fatalf("expected drift to be reported: %s", stdout2.String())
	}
}

// TestIntegration_Clone_LocalRepo clones a real, local source repository
// (a filesystem path stands in for the remote — no network, no GitHub)
// and checks the destination ends up correctly configured. Since
// CloneWithSSHCommand's destination is relative to the process's actual
// working directory (matching plain `git clone`'s own behavior), the test
// chdirs into a scratch directory for the duration of the clone.
func TestIntegration_Clone_LocalRepo(t *testing.T) {
	requireGit(t)

	src := t.TempDir()
	if out, err := exec.Command("git", "init", "--quiet", src).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	commit := exec.Command("git", "-C", src, "commit", "--allow-empty", "-m", "initial", "--quiet")
	commit.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}

	scratch := t.TempDir()
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(scratch); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(origWd)

	accounts := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", GitName: "Cristhian Delgado",
		GitEmail: "cristhian@leantech.com",
	})
	account, _ := accounts.FindByID("lean-tech")
	mechanism := app.NewSSHCommandMechanism()

	// ParseRepositoryURL/host validation only matter for real github.com
	// URLs (already covered by the CLI-level fake tests); this test
	// exercises CloneService's actual git plumbing end to end, so it
	// drives GitClient/mechanism directly against a source that's a plain
	// filesystem path rather than routing a fake URL through the CLI.
	dest := filepath.Join(scratch, "cloned")
	if err := infra.NewGitClient("").CloneWithSSHCommand(src, dest, ""); err != nil {
		t.Fatalf("CloneWithSSHCommand: %v", err)
	}
	if err := mechanism.Apply(infra.NewGitClient(dest), *account); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	out, err := exec.Command("git", "-C", dest, "config", "--local", "user.email").CombinedOutput()
	if err != nil {
		t.Fatalf("git config: %v: %s", err, out)
	}
	if strings.TrimSpace(string(out)) != "cristhian@leantech.com" {
		t.Fatalf("got %q", out)
	}
}
