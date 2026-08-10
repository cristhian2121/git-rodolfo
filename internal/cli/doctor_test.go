package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func doctorDeps(t *testing.T, accounts *fakes.FakeAccountRepository, ssh *fakes.FakeSSHClient, env *fakes.FakeEnvironmentInspector, git *fakes.FakeGitClient) (cli.Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var newGitClient func(dir string) app.GitClient
	if git != nil {
		newGitClient = func(dir string) app.GitClient { return git }
	} else {
		newGitClient = func(dir string) app.GitClient { return fakes.NewFakeGitClient() } // IsRepo defaults false
	}
	return cli.Deps{
		Accounts:     app.NewAccountService(accounts),
		AccountRepo:  accounts,
		SSH:          ssh,
		Env:          env,
		Mechanism:    app.NewSSHCommandMechanism(),
		NewGitClient: newGitClient,
		Getwd:        func() (string, error) { return "/repo/project", nil },
		Stdout:       &stdout,
		Stderr:       &stderr,
		Now:          fixedNow,
	}, &stdout, &stderr
}

func TestRun_Doctor_AllHealthy(t *testing.T) {
	accounts := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", ProviderUsername: "cristhiandelgado-work", PrivateKeyPath: "/k",
	})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/k"] = fakes.FakeKey{Valid: true, AuthResult: domain.AuthResult{Success: true, Username: "cristhiandelgado-work"}}
	deps, stdout, stderr := doctorDeps(t, accounts, ssh, fakes.NewFakeEnvironmentInspector(), nil)

	code := cli.Run([]string{"doctor"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Git Rodolfo Doctor") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "No issues found.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "✗") {
		t.Fatalf("did not expect any failing marks: %s", stdout.String())
	}
}

func TestRun_Doctor_ReportsIssuesAndExitsNonZero(t *testing.T) {
	accounts := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", ProviderUsername: "cristhiandelgado-work", PrivateKeyPath: "/k",
	})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/k"] = fakes.FakeKey{Valid: true, AuthResult: domain.AuthResult{Success: false}}
	deps, stdout, _ := doctorDeps(t, accounts, ssh, fakes.NewFakeEnvironmentInspector(), nil)

	code := cli.Run([]string{"doctor"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "1 issue found.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "✗ Lean Tech authentication failed") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Possible causes:") {
		t.Fatalf("expected causes to be listed: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Run: git rodolfo account show lean-tech") {
		t.Fatalf("expected the suggested command: %s", stdout.String())
	}
}

// TestRun_Doctor_RendersMultiLineCommand covers RF-46's Windows fix,
// which is two icacls invocations (no single operator chains them
// correctly in both cmd.exe and PowerShell) — doctor must print each line
// as its own command instead of squashing them onto one "Run:" line.
func TestRun_Doctor_RendersMultiLineCommand(t *testing.T) {
	accounts := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", ProviderUsername: "cristhiandelgado-work", PrivateKeyPath: "/k",
	})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/k"] = fakes.FakeKey{
		Valid:           true,
		AuthResult:      domain.AuthResult{Success: true, Username: "cristhiandelgado-work"},
		PermissionCause: `permissions are too open: "Everyone" has explicit access`,
		PermissionFix:   "icacls \"/k\" /reset\nicacls \"/k\" /inheritance:r /grant:r \"user\":F",
	}
	deps, stdout, _ := doctorDeps(t, accounts, ssh, fakes.NewFakeEnvironmentInspector(), nil)

	code := cli.Run([]string{"doctor"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "Run:\n  icacls \"/k\" /reset\n  icacls \"/k\" /inheritance:r /grant:r \"user\":F\n") {
		t.Fatalf("expected each command on its own line: %s", stdout.String())
	}
}

func TestRun_Doctor_TakesNoArguments(t *testing.T) {
	deps, _, stderr := doctorDeps(t, fakes.NewFakeAccountRepository(), fakes.NewFakeSSHClient(), fakes.NewFakeEnvironmentInspector(), nil)

	code := cli.Run([]string{"doctor", "extra"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "takes no arguments") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRun_Doctor_IncludesRepoChecksWhenInsideARepository(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.IsRepo = true
	accounts := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "e@example.com"})
	account, _ := accounts.FindByID("lean-tech")
	if err := app.NewSSHCommandMechanism().Apply(git, *account); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	deps, stdout, stderr := doctorDeps(t, accounts, fakes.NewFakeSSHClient(), fakes.NewFakeEnvironmentInspector(), git)

	code := cli.Run([]string{"doctor"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `Current repository matches account "Lean Tech"`) {
		t.Fatalf("expected a repository finding: %s", stdout.String())
	}
}
