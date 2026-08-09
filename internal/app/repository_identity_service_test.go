package app_test

import (
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func TestRepositoryIdentityService_UseAndClear(t *testing.T) {
	git := fakes.NewFakeGitClient()
	repo := fakes.NewFakeAccountRepository()
	svc := app.NewRepositoryIdentityService(git, app.NewSSHCommandMechanism(), repo)
	account := domain.Account{ID: "lean-tech", GitName: "N", GitEmail: "e@example.com", PrivateKeyPath: "/k"}

	if err := svc.Use(account); err != nil {
		t.Fatalf("Use: %v", err)
	}
	if git.LocalConfig["rodolfo.account"] != "lean-tech" {
		t.Fatalf("rodolfo.account = %q", git.LocalConfig["rodolfo.account"])
	}

	if err := svc.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if len(git.LocalConfig) != 0 {
		t.Fatalf("expected config cleared, got %v", git.LocalConfig)
	}
}

func TestRepositoryIdentityService_RemoteURL_None(t *testing.T) {
	git := fakes.NewFakeGitClient()
	svc := app.NewRepositoryIdentityService(git, app.NewSSHCommandMechanism(), fakes.NewFakeAccountRepository())

	url, has, err := svc.RemoteURL()
	if err != nil {
		t.Fatalf("RemoteURL: %v", err)
	}
	if has || url != "" {
		t.Fatalf("expected no remote, got url=%q has=%v", url, has)
	}
}

func TestRepositoryIdentityService_Current_NotManaged(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.GlobalConfig["user.email"] = "cristhian@gmail.com"
	svc := app.NewRepositoryIdentityService(git, app.NewSSHCommandMechanism(), fakes.NewFakeAccountRepository())

	status, err := svc.Current()
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if status.Managed {
		t.Fatal("expected not managed")
	}
	if status.EffectiveGitEmail != "cristhian@gmail.com" {
		t.Fatalf("EffectiveGitEmail = %q", status.EffectiveGitEmail)
	}
}

func TestRepositoryIdentityService_Current_ManagedAndConsistent(t *testing.T) {
	git := fakes.NewFakeGitClient()
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "e@example.com"})
	mechanism := app.NewSSHCommandMechanism()
	account, _ := repo.FindByID("lean-tech")
	if err := mechanism.Apply(git, *account); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	git.RemoteURLs["origin"] = "git@github.com:lean-tech/project.git"

	svc := app.NewRepositoryIdentityService(git, mechanism, repo)
	status, err := svc.Current()
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if !status.Managed {
		t.Fatal("expected managed")
	}
	if !status.Validation.OK {
		t.Fatalf("expected consistent, got issues: %v", status.Validation.Issues)
	}
	if status.RemoteURL != "git@github.com:lean-tech/project.git" || !status.HasRemote {
		t.Fatalf("unexpected remote: %+v", status)
	}
}

func TestRepositoryIdentityService_Current_Inconsistent(t *testing.T) {
	git := fakes.NewFakeGitClient()
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "cristhian@leantech.com"})
	mechanism := app.NewSSHCommandMechanism()
	account, _ := repo.FindByID("lean-tech")
	if err := mechanism.Apply(git, *account); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	git.LocalConfig["user.email"] = "cristhian@gmail.com" // drifted

	svc := app.NewRepositoryIdentityService(git, mechanism, repo)
	status, err := svc.Current()
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if status.Validation.OK {
		t.Fatal("expected inconsistency to be detected")
	}
}

func TestRepositoryIdentityService_Current_AssignedAccountDeleted(t *testing.T) {
	git := fakes.NewFakeGitClient()
	git.LocalConfig["rodolfo.account"] = "ghost"
	repo := fakes.NewFakeAccountRepository() // "ghost" doesn't exist

	svc := app.NewRepositoryIdentityService(git, app.NewSSHCommandMechanism(), repo)
	status, err := svc.Current()
	if err != nil {
		t.Fatalf("Current: %v", err)
	}
	if !status.Managed || !status.AccountDeleted || status.AssignedID != "ghost" {
		t.Fatalf("unexpected status: %+v", status)
	}
}
