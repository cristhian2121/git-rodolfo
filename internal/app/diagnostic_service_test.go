package app_test

import (
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func findingFor(t *testing.T, findings []app.DiagnosticFinding, substr string) app.DiagnosticFinding {
	t.Helper()
	for _, f := range findings {
		if len(f.Message) >= len(substr) && contains(f.Message, substr) {
			return f
		}
	}
	t.Fatalf("no finding found containing %q in %+v", substr, findings)
	return app.DiagnosticFinding{}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestDiagnosticService_AllHealthy(t *testing.T) {
	accounts := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", ProviderUsername: "cristhiandelgado-work",
		Hostname: "github.com", PrivateKeyPath: "/k", PublicKeyFingerprint: "SHA256:aaa",
	})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/k"] = fakes.FakeKey{
		Valid:       true,
		Fingerprint: "SHA256:aaa",
		AuthResult:  domain.AuthResult{Success: true, Username: "cristhiandelgado-work"},
	}
	env := fakes.NewFakeEnvironmentInspector()
	svc := app.NewDiagnosticService(accounts, ssh, env, nil, nil)

	findings, err := svc.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if !f.OK {
			t.Fatalf("expected all findings OK, got failing: %+v", f)
		}
	}
	// Sanity: every validation category produced something.
	findingFor(t, findings, "Git 2.43.0 installed")
	findingFor(t, findings, "SSH installed")
	findingFor(t, findings, "Lean Tech key found (/k)")
	findingFor(t, findings, "Lean Tech authenticates as cristhiandelgado-work")
	findingFor(t, findings, "No shared keys between accounts")
	findingFor(t, findings, "GIT_SSH_COMMAND is not set")
}

func TestDiagnosticService_GitNotInstalled(t *testing.T) {
	env := fakes.NewFakeEnvironmentInspector()
	env.GitVersionErr = errNotFound
	svc := app.NewDiagnosticService(fakes.NewFakeAccountRepository(), fakes.NewFakeSSHClient(), env, nil, nil)

	findings, err := svc.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	f := findingFor(t, findings, "Git installation")
	if f.OK {
		t.Fatal("expected Git installation to fail")
	}
}

func TestDiagnosticService_MissingKey(t *testing.T) {
	accounts := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/missing"})
	svc := app.NewDiagnosticService(accounts, fakes.NewFakeSSHClient(), fakes.NewFakeEnvironmentInspector(), nil, nil)

	findings, err := svc.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	f := findingFor(t, findings, "Lean Tech key found")
	if f.OK {
		t.Fatal("expected the missing key to be flagged")
	}
	if f.Command != "git rodolfo account edit lean-tech" {
		t.Fatalf("unexpected command: %q", f.Command)
	}
}

func TestDiagnosticService_InsecurePermissions(t *testing.T) {
	accounts := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PrivateKeyPath: "/k"})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/k"] = fakes.FakeKey{Valid: true, PermissionCause: "permissions are too open (0644); private keys should be 600", PermissionFix: "chmod 600 /k"}
	svc := app.NewDiagnosticService(accounts, ssh, fakes.NewFakeEnvironmentInspector(), nil, nil)

	findings, err := svc.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	f := findingFor(t, findings, "Lean Tech key found")
	if f.OK {
		t.Fatal("expected insecure permissions to be flagged")
	}
	if f.Command != "chmod 600 /k" {
		t.Fatalf("unexpected command: %q", f.Command)
	}
}

func TestDiagnosticService_AuthenticationFailure(t *testing.T) {
	accounts := fakes.NewFakeAccountRepository(domain.Account{
		ID: "lean-tech", DisplayName: "Lean Tech", ProviderUsername: "cristhiandelgado-work",
		PrivateKeyPath: "/k",
	})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/k"] = fakes.FakeKey{Valid: true, AuthResult: domain.AuthResult{Success: false}}
	svc := app.NewDiagnosticService(accounts, ssh, fakes.NewFakeEnvironmentInspector(), nil, nil)

	findings, err := svc.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	f := findingFor(t, findings, "Lean Tech authentication failed")
	if f.OK {
		t.Fatal("expected authentication failure to be flagged")
	}
	if len(f.Causes) == 0 {
		t.Fatal("expected possible causes to be listed")
	}
}

func TestDiagnosticService_SharedKeyDetected(t *testing.T) {
	accounts := fakes.NewFakeAccountRepository(
		domain.Account{ID: "personal", DisplayName: "Personal", PublicKeyFingerprint: "SHA256:shared"},
		domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", PublicKeyFingerprint: "SHA256:shared"},
	)
	svc := app.NewDiagnosticService(accounts, fakes.NewFakeSSHClient(), fakes.NewFakeEnvironmentInspector(), nil, nil)

	findings, err := svc.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	f := findingFor(t, findings, "Shared keys between accounts")
	if f.OK {
		t.Fatal("expected a shared key to be flagged")
	}
}

func TestDiagnosticService_GitSSHCommandEnvSet(t *testing.T) {
	env := fakes.NewFakeEnvironmentInspector()
	env.Env["GIT_SSH_COMMAND"] = "ssh -i /some/other/key"
	svc := app.NewDiagnosticService(fakes.NewFakeAccountRepository(), fakes.NewFakeSSHClient(), env, nil, nil)

	findings, err := svc.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	f := findingFor(t, findings, "GIT_SSH_COMMAND environment variable")
	if f.OK {
		t.Fatal("expected GIT_SSH_COMMAND override to be flagged")
	}
	if f.Command != "unset GIT_SSH_COMMAND" {
		t.Fatalf("unexpected command: %q", f.Command)
	}
}

func TestDiagnosticService_CurrentRepositoryConsistent(t *testing.T) {
	accounts := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "e@example.com"})
	git := fakes.NewFakeGitClient()
	mechanism := app.NewSSHCommandMechanism()
	account, _ := accounts.FindByID("lean-tech")
	if err := mechanism.Apply(git, *account); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	git.RemoteURLs["origin"] = "git@github.com:lean-tech/project.git"

	svc := app.NewDiagnosticService(accounts, fakes.NewFakeSSHClient(), fakes.NewFakeEnvironmentInspector(), git, mechanism)
	findings, err := svc.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	f := findingFor(t, findings, `Current repository matches account "Lean Tech"`)
	if !f.OK {
		t.Fatalf("expected consistent repo, got: %+v", f)
	}
	hostFinding := findingFor(t, findings, "Remote host supported")
	if !hostFinding.OK {
		t.Fatalf("expected github.com to be a supported host: %+v", hostFinding)
	}
}

func TestDiagnosticService_CurrentRepositoryDrifted(t *testing.T) {
	accounts := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech", GitEmail: "cristhian@leantech.com"})
	git := fakes.NewFakeGitClient()
	mechanism := app.NewSSHCommandMechanism()
	account, _ := accounts.FindByID("lean-tech")
	if err := mechanism.Apply(git, *account); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	git.LocalConfig["user.email"] = "cristhian@gmail.com" // drifted

	svc := app.NewDiagnosticService(accounts, fakes.NewFakeSSHClient(), fakes.NewFakeEnvironmentInspector(), git, mechanism)
	findings, err := svc.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	f := findingFor(t, findings, `Current repository matches account "Lean Tech"`)
	if f.OK {
		t.Fatal("expected drift to be detected")
	}
}

func TestDiagnosticService_UnmanagedRepoSkipsRepoChecks(t *testing.T) {
	accounts := fakes.NewFakeAccountRepository()
	git := fakes.NewFakeGitClient() // rodolfo.account not set
	svc := app.NewDiagnosticService(accounts, fakes.NewFakeSSHClient(), fakes.NewFakeEnvironmentInspector(), git, app.NewSSHCommandMechanism())

	findings, err := svc.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if contains(f.Message, "Current repository") {
			t.Fatalf("did not expect a repository finding for an unmanaged repo: %+v", f)
		}
	}
}

func TestDiagnosticService_NotInARepo_SkipsRepoChecksEntirely(t *testing.T) {
	svc := app.NewDiagnosticService(fakes.NewFakeAccountRepository(), fakes.NewFakeSSHClient(), fakes.NewFakeEnvironmentInspector(), nil, nil)
	findings, err := svc.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for _, f := range findings {
		if contains(f.Message, "Current repository") || contains(f.Message, "Remote host") {
			t.Fatalf("did not expect repo-scoped findings when git is nil: %+v", f)
		}
	}
}

var errNotFound = fakeNotFoundError{}

type fakeNotFoundError struct{}

func (fakeNotFoundError) Error() string { return "git: command not found" }
