package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

func fixedClock() time.Time { return time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC) }

func newAddService(repo *fakes.FakeAccountRepository, ssh *fakes.FakeSSHClient, agent *fakes.FakeSSHAgentAdapter) *app.AccountAddService {
	return app.NewAccountAddService(repo, ssh, agent, fixedClock)
}

func TestAccountAddService_ConfigureLater(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	svc := newAddService(repo, fakes.NewFakeSSHClient(), fakes.NewFakeSSHAgentAdapter())

	acc, err := svc.Add(app.AccountAddInput{
		DisplayName:      "Lean Tech",
		ProviderUsername: "cristhiandelgado-work",
		GitName:          "Cristhian Delgado",
		GitEmail:         "cristhian@leantech.com",
		KeyMode:          app.KeyModeConfigureLater,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if acc.ID != "lean-tech" {
		t.Fatalf("id = %q, want lean-tech", acc.ID)
	}
	if acc.PrivateKeyPath != "" {
		t.Fatalf("expected no key path, got %q", acc.PrivateKeyPath)
	}
	if acc.AuthenticationStatus != domain.AuthenticationStatusUnverified {
		t.Fatalf("status = %q, want unverified", acc.AuthenticationStatus)
	}

	saved, _ := repo.FindByID("lean-tech")
	if saved == nil {
		t.Fatal("account was not persisted")
	}
}

func TestAccountAddService_GenerateKey(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	ssh := fakes.NewFakeSSHClient()
	agent := fakes.NewFakeSSHAgentAdapter()
	svc := newAddService(repo, ssh, agent)

	ssh.GenerateFingerprint = "SHA256:aaa" // stands in for ssh-keygen's real output

	acc, err := svc.Add(app.AccountAddInput{
		DisplayName:      "Lean Tech",
		ProviderUsername: "cristhiandelgado-work",
		GitName:          "Cristhian Delgado",
		GitEmail:         "cristhian@leantech.com",
		KeyMode:          app.KeyModeGenerate,
		KeyPath:          "/home/u/.ssh/id_ed25519_leantech",
		WithPassphrase:   false,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if acc.PublicKeyFingerprint != "SHA256:aaa" {
		t.Fatalf("fingerprint = %q", acc.PublicKeyFingerprint)
	}
	if len(ssh.GeneratedKeys) != 1 || ssh.GeneratedKeys[0].Path != "/home/u/.ssh/id_ed25519_leantech" {
		t.Fatalf("GenerateKey not called as expected: %+v", ssh.GeneratedKeys)
	}
}

func TestAccountAddService_ExistingKey_DuplicateFingerprintRejected(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID:                   "personal",
		DisplayName:          "Personal",
		PublicKeyFingerprint: "SHA256:shared",
	})
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/home/u/.ssh/id_ed25519"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:shared"}
	svc := newAddService(repo, ssh, fakes.NewFakeSSHAgentAdapter())

	_, err := svc.Add(app.AccountAddInput{
		DisplayName:      "Lean Tech",
		ProviderUsername: "cristhiandelgado-work",
		GitName:          "Cristhian Delgado",
		GitEmail:         "cristhian@leantech.com",
		KeyMode:          app.KeyModeExisting,
		KeyPath:          "/home/u/.ssh/id_ed25519",
	})

	var dupErr *app.KeyAlreadyInUseError
	if !errors.As(err, &dupErr) {
		t.Fatalf("expected KeyAlreadyInUseError, got %v", err)
	}
	if dupErr.OtherAccountName != "Personal" {
		t.Fatalf("OtherAccountName = %q, want Personal", dupErr.OtherAccountName)
	}

	if accounts, _ := repo.List(); len(accounts) != 1 {
		t.Fatalf("expected no new account to be saved, got %d", len(accounts))
	}
}

func TestAccountAddService_ExistingKey_InvalidRejected(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	svc := newAddService(repo, fakes.NewFakeSSHClient(), fakes.NewFakeSSHAgentAdapter())

	_, err := svc.Add(app.AccountAddInput{
		DisplayName:      "Lean Tech",
		ProviderUsername: "cristhiandelgado-work",
		GitName:          "Cristhian Delgado",
		GitEmail:         "cristhian@leantech.com",
		KeyMode:          app.KeyModeExisting,
		KeyPath:          "/home/u/.ssh/does-not-exist",
	})
	if err == nil {
		t.Fatal("expected an error validating a nonexistent key")
	}
}

func TestAccountAddService_PassphraseKeyLoadsIntoAgent(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/home/u/.ssh/id_protected"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:bbb", HasPassphrase: true}
	agent := fakes.NewFakeSSHAgentAdapter()
	svc := newAddService(repo, ssh, agent)

	_, err := svc.Add(app.AccountAddInput{
		DisplayName:      "Lean Tech",
		ProviderUsername: "cristhiandelgado-work",
		GitName:          "Cristhian Delgado",
		GitEmail:         "cristhian@leantech.com",
		KeyMode:          app.KeyModeExisting,
		KeyPath:          "/home/u/.ssh/id_protected",
		LoadIntoAgent:    true,
		UseKeychain:      true,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !agent.LoadedKeys["/home/u/.ssh/id_protected"] {
		t.Fatal("expected key to be loaded into the agent")
	}
}

func TestAccountAddService_NoPassphraseSkipsAgent(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	ssh := fakes.NewFakeSSHClient()
	ssh.Keys["/home/u/.ssh/id_plain"] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:ccc", HasPassphrase: false}
	agent := fakes.NewFakeSSHAgentAdapter()
	svc := newAddService(repo, ssh, agent)

	_, err := svc.Add(app.AccountAddInput{
		DisplayName:      "Lean Tech",
		ProviderUsername: "cristhiandelgado-work",
		GitName:          "Cristhian Delgado",
		GitEmail:         "cristhian@leantech.com",
		KeyMode:          app.KeyModeExisting,
		KeyPath:          "/home/u/.ssh/id_plain",
		LoadIntoAgent:    true,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(agent.LoadedKeys) != 0 {
		t.Fatalf("expected no agent load for a passphrase-less key, got %v", agent.LoadedKeys)
	}
}

func TestAccountAddService_RejectsUnsupportedProvider(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	svc := newAddService(repo, fakes.NewFakeSSHClient(), fakes.NewFakeSSHAgentAdapter())

	_, err := svc.Add(app.AccountAddInput{
		DisplayName:      "Some GitLab Account",
		ProviderUsername: "u",
		GitName:          "n",
		GitEmail:         "e@example.com",
		Provider:         "gitlab",
		KeyMode:          app.KeyModeConfigureLater,
	})
	if !errors.Is(err, app.ErrUnsupportedProvider) {
		t.Fatalf("expected ErrUnsupportedProvider, got %v", err)
	}
}

func TestAccountAddService_RejectsDuplicateAccountName(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{ID: "lean-tech", DisplayName: "Lean Tech"})
	svc := newAddService(repo, fakes.NewFakeSSHClient(), fakes.NewFakeSSHAgentAdapter())

	_, err := svc.Add(app.AccountAddInput{
		DisplayName:      "Lean Tech",
		ProviderUsername: "u",
		GitName:          "n",
		GitEmail:         "e@example.com",
		KeyMode:          app.KeyModeConfigureLater,
	})
	if !errors.Is(err, app.ErrAccountAlreadyExists) {
		t.Fatalf("expected ErrAccountAlreadyExists, got %v", err)
	}
}

// TestAccountAddService_RejectsKeyPathWithSingleQuote is a regression test
// for a shell-injection vulnerability: SSHCommandMechanism embeds the key
// path in a single-quoted core.sshCommand value that Git later runs
// through a shell, so a path containing a single quote could break out of
// the quoting and inject arbitrary commands that persist in .git/config.
func TestAccountAddService_RejectsKeyPathWithSingleQuote(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	ssh := fakes.NewFakeSSHClient()
	svc := newAddService(repo, ssh, fakes.NewFakeSSHAgentAdapter())

	maliciousPath := "/home/u/.ssh/id_ed25519'; touch /tmp/PWNED; echo '"
	ssh.Keys[maliciousPath] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:evil"}

	_, err := svc.Add(app.AccountAddInput{
		DisplayName:      "Lean Tech",
		ProviderUsername: "u",
		GitName:          "n",
		GitEmail:         "e@example.com",
		KeyMode:          app.KeyModeExisting,
		KeyPath:          maliciousPath,
	})
	if !errors.Is(err, app.ErrUnsafeKeyPath) {
		t.Fatalf("expected ErrUnsafeKeyPath, got %v", err)
	}
	if accounts, _ := repo.List(); len(accounts) != 0 {
		t.Fatal("account must not be saved when the key path is rejected")
	}
}

func TestAccountAddService_RequiresMandatoryFields(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	svc := newAddService(repo, fakes.NewFakeSSHClient(), fakes.NewFakeSSHAgentAdapter())

	_, err := svc.Add(app.AccountAddInput{DisplayName: "Lean Tech"})
	if err == nil {
		t.Fatal("expected an error for missing required fields")
	}
}
