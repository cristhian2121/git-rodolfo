package cli_test

import (
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func TestRun_Current_NotManaged(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.IsRepo = true
	git.GlobalConfig["user.email"] = "cristhian@gmail.com"
	deps, stdout, stderr := repoDeps(t, nil, fakes.NewFakeAccountRepository(), git)

	code := cli.Run([]string{"current"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "This repository is not managed by Git Rodolfo.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "cristhian@gmail.com") {
		t.Fatalf("expected global identity to be shown: %s", stdout.String())
	}
}

func TestRun_Current_ManagedAndConsistent(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.IsRepo = true
	git.RemoteURLs["origin"] = "git@github.com:lean-tech/project.git"
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", GitName: "Cristhian Delgado",
		GitEmail: "cristhian@leantech.com", PrivateKeyPath: "/k",
	})
	account, _ := repo.FindByID("lean-tech")
	if err := app.NewSSHCommandMechanism().Apply(git, *account); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	deps, stdout, stderr := repoDeps(t, nil, repo, git)

	code := cli.Run([]string{"current"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Account: Lean Tech") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Status: correctly configured") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestRun_Current_Inconsistent(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.IsRepo = true
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "cristhian@leantech.com",
	})
	account, _ := repo.FindByID("lean-tech")
	if err := app.NewSSHCommandMechanism().Apply(git, *account); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	git.LocalConfig["user.email"] = "cristhian@gmail.com" // drifted

	deps, stdout, stderr := repoDeps(t, nil, repo, git)

	code := cli.Run([]string{"current"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Warning:") {
		t.Fatalf("expected a warning block: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "git rodolfo use lean-tech") {
		t.Fatalf("expected the suggested fix command: %s", stdout.String())
	}
}

func TestRun_Current_AssignedAccountDeleted(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.IsRepo = true
	git.LocalConfig["rodolfo.account"] = "ghost"
	deps, stdout, stderr := repoDeps(t, nil, fakes.NewFakeAccountRepository(), git)

	code := cli.Run([]string{"current"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"ghost"`) {
		t.Fatalf("expected the missing account id to be mentioned: %s", stdout.String())
	}
}

func TestRun_Current_NotARepository(t *testing.T) {
	git := fakes.NewFakeGitClient()
	deps, _, stderr := repoDeps(t, nil, fakes.NewFakeAccountRepository(), git)

	code := cli.Run([]string{"current"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "not a Git repository") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}
