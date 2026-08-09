package cli_test

import (
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/cli"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func TestRun_Use_NotARepository(t *testing.T) {
	git := fakes.NewFakeGitClient() // IsRepo defaults to false
	deps, _, stderr := repoDeps(t, nil, fakes.NewFakeAccountRepository(), git)

	code := cli.Run([]string{"use", "lean-tech", "--non-interactive"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "not a Git repository") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRun_Use_NonInteractive_WithRemote(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.IsRepo = true
	git.RemoteURLs["origin"] = "git@github.com:lean-tech/project.git"
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "cristhian@leantech.com", PrivateKeyPath: "/k",
	})
	deps, stdout, stderr := repoDeps(t, nil, repo, git)

	code := cli.Run([]string{"use", "lean-tech", "--non-interactive"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Repository identity updated.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "git@github.com:lean-tech/project.git (unchanged)") {
		t.Fatalf("expected remote to be reported unchanged: %s", stdout.String())
	}
	if git.LocalConfig["rodolfo.account"] != "lean-tech" {
		t.Fatalf("rodolfo.account = %q", git.LocalConfig["rodolfo.account"])
	}
	if git.LocalConfig["core.sshCommand"] == "" {
		t.Fatal("expected core.sshCommand to be set")
	}
}

func TestRun_Use_NoRemote(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.IsRepo = true // no remote configured
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "e@example.com"})
	deps, stdout, stderr := repoDeps(t, nil, repo, git)

	code := cli.Run([]string{"use", "lean-tech", "--non-interactive"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Remote: none") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestRun_Use_Clear(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.IsRepo = true
	git.LocalConfig["user.name"] = "N"
	git.LocalConfig["user.email"] = "e@example.com"
	git.LocalConfig["rodolfo.account"] = "lean-tech"
	git.LocalConfig["core.sshCommand"] = "ssh -i '/k' -o IdentitiesOnly=yes"
	deps, stdout, stderr := repoDeps(t, nil, fakes.NewFakeAccountRepository(), git)

	code := cli.Run([]string{"use", "--clear"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Repository identity cleared.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if len(git.LocalConfig) != 0 {
		t.Fatalf("expected all config cleared, got %v", git.LocalConfig)
	}
}

func TestRun_Use_RequiresAccountInNonInteractive(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.IsRepo = true
	deps, _, stderr := repoDeps(t, nil, fakes.NewFakeAccountRepository(), git)

	code := cli.Run([]string{"use", "--non-interactive"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires an account id") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRun_Use_WarnsOnUnsupportedRemoteHost(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.IsRepo = true
	git.RemoteURLs["origin"] = "git@gitlab.com:team/project.git"
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "e@example.com"})
	deps, stdout, stderr := repoDeps(t, nil, repo, git)

	code := cli.Run([]string{"use", "lean-tech", "--non-interactive"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `warning: host "gitlab.com" is not supported`) {
		t.Fatalf("expected a host warning, got: %s", stdout.String())
	}
}
