package cli_test

import (
	"os"
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"

	"github.com/lean-tech/git-rodolfo/internal/cli"
)

// TestRun_Clone_TranslatesRepositoryNotFound is RF-21 / CU-07 end to end
// through the CLI: a "Repository not found" clone failure must come back
// as the translated explanation, not the raw Git error.
func TestRun_Clone_TranslatesRepositoryNotFound(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.CloneErr = &app.GitCommandError{
		Output: "ERROR: Repository not found.\nfatal: Could not read from remote repository.",
	}
	repo := fakes.NewFakeAccountRepository(
		domain.Account{ID: "personal", DisplayName: "Personal", ProviderUsername: "cristhiandelgado"},
		domain.Account{ID: "lean-tech", DisplayName: "Lean Tech"},
	)
	deps, _, stderr := repoDeps(t, nil, repo, git)

	code := cli.Run([]string{
		"clone", "git@github.com:lean-tech/project.git",
		"--account", "personal", "--non-interactive",
	}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	for _, want := range []string{
		"GitHub says the repository was not found.",
		"You authenticated as: cristhiandelgado (Personal)",
		"The repository belongs to: lean-tech",
		`You have an account for that organization: "Lean Tech"`,
		"Original error:",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestRun_Clone_NonInteractive_SSH(t *testing.T) {
	git := fakes.NewFakeGitClient()
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", GitName: "Cristhian Delgado",
		GitEmail: "cristhian@leantech.com", PrivateKeyPath: "/k",
	})
	deps, stdout, stderr := repoDeps(t, nil, repo, git)

	code := cli.Run([]string{
		"clone", "git@github.com:lean-tech/project.git",
		"--account", "lean-tech", "--non-interactive",
	}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Repository cloned successfully.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if len(git.ClonedCalls) != 1 || git.ClonedCalls[0].Destination != "project" {
		t.Fatalf("unexpected clone calls: %+v", git.ClonedCalls)
	}
	if git.ClonedCalls[0].SSHCommand == "" {
		t.Fatal("expected core.sshCommand to be forced for an SSH URL")
	}
	if git.LocalConfig["rodolfo.account"] != "lean-tech" {
		t.Fatalf("rodolfo.account = %q", git.LocalConfig["rodolfo.account"])
	}
}

func TestRun_Clone_RejectsUnsupportedHost(t *testing.T) {
	git := fakes.NewFakeGitClient()
	deps, _, stderr := repoDeps(t, nil, fakes.NewFakeAccountRepository(), git)

	code := cli.Run([]string{
		"clone", "git@gitlab.com:team/project.git",
		"--account", "lean-tech", "--non-interactive",
	}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "Git Rodolfo only supports GitHub in this version.") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "git@gitlab.com:team/project.git") {
		t.Fatalf("expected the original remote to be quoted: %s", stderr.String())
	}
	if len(git.ClonedCalls) != 0 {
		t.Fatal("no clone should have been attempted")
	}
}

func TestRun_Clone_HTTPS_NonInteractiveRequiresExplicitChoice(t *testing.T) {
	git := fakes.NewFakeGitClient()
	deps, _, stderr := repoDeps(t, nil, fakes.NewFakeAccountRepository(), git)

	code := cli.Run([]string{
		"clone", "https://github.com/lean-tech/project.git",
		"--account", "lean-tech", "--non-interactive",
	}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--keep-https") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRun_Clone_HTTPS_KeepHTTPS(t *testing.T) {
	git := fakes.NewFakeGitClient()
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", GitName: "N", GitEmail: "e@example.com", PrivateKeyPath: "/k",
	})
	deps, stdout, stderr := repoDeps(t, nil, repo, git)

	code := cli.Run([]string{
		"clone", "https://github.com/lean-tech/project.git",
		"--account", "lean-tech", "--non-interactive", "--keep-https",
	}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if git.ClonedCalls[0].SSHCommand != "" {
		t.Fatalf("expected no sshCommand forced, got %q", git.ClonedCalls[0].SSHCommand)
	}
	if _, ok := git.LocalConfig["core.sshCommand"]; ok {
		t.Fatal("expected core.sshCommand to be skipped when keeping HTTPS")
	}
	if !strings.Contains(stdout.String(), "credential helper") {
		t.Fatalf("expected a note about the credential helper: %s", stdout.String())
	}
}

func TestRun_Clone_RequiresAccountInNonInteractive(t *testing.T) {
	git := fakes.NewFakeGitClient()
	deps, _, stderr := repoDeps(t, nil, fakes.NewFakeAccountRepository(), git)

	code := cli.Run([]string{"clone", "git@github.com:lean-tech/project.git", "--non-interactive"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "requires --account") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

// TestRun_Clone_Interactive_SelectAccountAndConvertHTTPS drives both
// interactive decision points (HTTPS->SSH choice and account selection)
// through one pipe, covering RF-16's required confirmation end to end.
func TestRun_Clone_Interactive_SelectAccountAndConvertHTTPS(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	git := fakes.NewFakeGitClient()
	repo := fakes.NewFakeAccountRepository(
		domain.Account{ID: "personal", DisplayName: "Personal"},
		domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", GitName: "N", GitEmail: "e@example.com", PrivateKeyPath: "/k"},
	)
	deps, stdout, stderr := repoDeps(t, r, repo, git)

	go func() {
		defer w.Close()
		w.WriteString("1\n") // "Clone using SSH"
		w.WriteString("2\n") // select "Lean Tech"
	}()

	code := cli.Run([]string{"clone", "https://github.com/lean-tech/project.git"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	if git.ClonedCalls[0].RemoteURL != "git@github.com:lean-tech/project.git" {
		t.Fatalf("expected the URL to be converted to SSH, got %q", git.ClonedCalls[0].RemoteURL)
	}
	if git.ClonedCalls[0].SSHCommand == "" {
		t.Fatal("expected SSH to be forced after choosing to convert")
	}
	if git.LocalConfig["rodolfo.account"] != "lean-tech" {
		t.Fatalf("rodolfo.account = %q", git.LocalConfig["rodolfo.account"])
	}
}
