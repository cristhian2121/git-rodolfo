package app_test

import (
	"errors"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func TestValidateHost_RejectsNonGitHub(t *testing.T) {
	u, err := domain.ParseRepositoryURL("git@gitlab.com:team/project.git")
	if err != nil {
		t.Fatalf("ParseRepositoryURL: %v", err)
	}

	err = app.ValidateHost(u, "git@gitlab.com:team/project.git")
	var unsupported *app.ErrUnsupportedHost
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected ErrUnsupportedHost, got %v", err)
	}
	if unsupported.Host != "gitlab.com" {
		t.Fatalf("Host = %q", unsupported.Host)
	}
}

func TestValidateHost_AcceptsGitHub(t *testing.T) {
	u, err := domain.ParseRepositoryURL("git@github.com:lean-tech/project.git")
	if err != nil {
		t.Fatalf("ParseRepositoryURL: %v", err)
	}
	if err := app.ValidateHost(u, "git@github.com:lean-tech/project.git"); err != nil {
		t.Fatalf("expected github.com to be accepted, got %v", err)
	}
}

func TestCloneService_Clone_ForceSSH(t *testing.T) {
	git := fakes.NewFakeGitClient()
	var boundDir string
	newGitForDir := func(dir string) app.GitClient {
		boundDir = dir
		return git // same fake instance is fine: we only assert on its LocalConfig/ClonedCalls
	}
	svc := app.NewCloneService(git, newGitForDir, app.NewSSHCommandMechanism())

	u, _ := domain.ParseRepositoryURL("git@github.com:lean-tech/project.git")
	account := domain.Account{ID: "lean-tech", GitName: "N", GitEmail: "e@example.com", PrivateKeyPath: "/k"}

	if err := svc.Clone(u, "project", account, true); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if boundDir != "project" {
		t.Fatalf("newGitForDir called with %q, want %q", boundDir, "project")
	}
	if len(git.ClonedCalls) != 1 {
		t.Fatalf("expected one clone call, got %d", len(git.ClonedCalls))
	}
	call := git.ClonedCalls[0]
	if call.RemoteURL != "git@github.com:lean-tech/project.git" || call.Destination != "project" {
		t.Fatalf("unexpected clone call: %+v", call)
	}
	if call.SSHCommand == "" {
		t.Fatal("expected core.sshCommand to be forced on the clone itself")
	}
	if git.LocalConfig["core.sshCommand"] == "" {
		t.Fatal("expected core.sshCommand persisted locally after clone")
	}
	if git.LocalConfig["rodolfo.account"] != "lean-tech" {
		t.Fatalf("rodolfo.account = %q", git.LocalConfig["rodolfo.account"])
	}
}

func TestCloneService_Clone_KeepHTTPS_SkipsSSHCommand(t *testing.T) {
	git := fakes.NewFakeGitClient()
	svc := app.NewCloneService(git, func(dir string) app.GitClient { return git }, app.NewSSHCommandMechanism())

	u, _ := domain.ParseRepositoryURL("https://github.com/lean-tech/project.git")
	account := domain.Account{ID: "lean-tech", GitName: "N", GitEmail: "e@example.com", PrivateKeyPath: "/k"}

	if err := svc.Clone(u, "project", account, false); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	if git.ClonedCalls[0].SSHCommand != "" {
		t.Fatalf("expected no sshCommand forced on the clone itself, got %q", git.ClonedCalls[0].SSHCommand)
	}
	if _, ok := git.LocalConfig["core.sshCommand"]; ok {
		t.Fatal("expected core.sshCommand to be skipped when keeping HTTPS")
	}
	// user.name/email/rodolfo.account must still be configured.
	if git.LocalConfig["rodolfo.account"] != "lean-tech" {
		t.Fatalf("rodolfo.account = %q", git.LocalConfig["rodolfo.account"])
	}
}
