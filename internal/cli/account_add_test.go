package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
	"github.com/lean-tech/git-rodolfo/internal/cli"
	"github.com/lean-tech/git-rodolfo/internal/domain"
	"github.com/lean-tech/git-rodolfo/internal/infra/fakes"
)

// addDeps builds a full Deps wired to fakes, including AccountAdd, for
// testing "account add". stdin, if non-nil, backs the interactive prompts.
func addDeps(t *testing.T, stdin *os.File, repo *fakes.FakeAccountRepository, ssh *fakes.FakeSSHClient, agent *fakes.FakeSSHAgentAdapter) (cli.Deps, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	return cli.Deps{
		Accounts:    app.NewAccountService(repo),
		AccountAdd:  app.NewAccountAddService(repo, ssh, agent, fixedNow),
		Auth:        app.NewAuthenticationService(ssh, repo, fixedNow),
		SSH:         ssh,
		Agent:       agent,
		Provider:    fakes.NewFakeProviderClient(),
		AccountRepo: repo,
		SSHDir:      t.TempDir(),
		Stdout:      &stdout,
		Stderr:      &stderr,
		Stdin:       stdin,
		Now:         fixedNow,
	}, &stdout, &stderr
}

func TestRun_AccountAddNonInteractive_ConfigureLater(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	deps, stdout, stderr := addDeps(t, nil, repo, fakes.NewFakeSSHClient(), fakes.NewFakeSSHAgentAdapter())

	code := cli.Run([]string{
		"account", "add",
		"--non-interactive",
		"--name", "Lean Tech",
		"--username", "cristhiandelgado-work",
		"--commit-name", "Cristhian Delgado",
		"--commit-email", "cristhian@leantech.com",
		"--configure-later",
	}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `Account "Lean Tech" saved.`) {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	saved, _ := repo.FindByID("lean-tech")
	if saved == nil {
		t.Fatal("account not persisted")
	}
	if saved.AuthenticationStatus != domain.AuthenticationStatusUnverified {
		t.Fatalf("status = %q, want unverified", saved.AuthenticationStatus)
	}
}

func TestRun_AccountAddNonInteractive_MissingRequiredFlag(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	deps, _, stderr := addDeps(t, nil, repo, fakes.NewFakeSSHClient(), fakes.NewFakeSSHAgentAdapter())

	code := cli.Run([]string{
		"account", "add", "--non-interactive",
		"--name", "Lean Tech",
		"--configure-later",
	}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "--username, --commit-name and --commit-email") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRun_AccountAddFlags_WithoutNonInteractiveFlagErrors(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	deps, _, stderr := addDeps(t, nil, repo, fakes.NewFakeSSHClient(), fakes.NewFakeSSHAgentAdapter())

	code := cli.Run([]string{"account", "add", "--name", "Lean Tech"}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "require --non-interactive") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRun_AccountAddNonInteractive_RequiresExactlyOneKeyMode(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	deps, _, stderr := addDeps(t, nil, repo, fakes.NewFakeSSHClient(), fakes.NewFakeSSHAgentAdapter())

	code := cli.Run([]string{
		"account", "add", "--non-interactive",
		"--name", "Lean Tech",
		"--username", "u",
		"--commit-name", "n",
		"--commit-email", "e@example.com",
		"--configure-later",
		"--generate-key",
	}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "exactly one of --key, --generate-key or --configure-later") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRun_AccountAddNonInteractive_GenerateKey(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	ssh := fakes.NewFakeSSHClient()
	ssh.GenerateFingerprint = "SHA256:generated"
	deps, stdout, stderr := addDeps(t, nil, repo, ssh, fakes.NewFakeSSHAgentAdapter())

	code := cli.Run([]string{
		"account", "add", "--non-interactive",
		"--name", "Lean Tech",
		"--username", "cristhiandelgado-work",
		"--commit-name", "Cristhian Delgado",
		"--commit-email", "cristhian@leantech.com",
		"--generate-key",
	}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `saved`) {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	saved, _ := repo.FindByID("lean-tech")
	if saved == nil || saved.PublicKeyFingerprint != "SHA256:generated" {
		t.Fatalf("unexpected saved account: %+v", saved)
	}
	if len(ssh.GeneratedKeys) != 1 {
		t.Fatalf("expected exactly one GenerateKey call, got %d", len(ssh.GeneratedKeys))
	}
	if ssh.GeneratedKeys[0].WithPassphrase {
		t.Fatal("non-interactive generate must never request a passphrase (RF-11)")
	}
	if !strings.HasSuffix(ssh.GeneratedKeys[0].Path, "id_ed25519_lean-tech") {
		t.Fatalf("unexpected default key path: %s", ssh.GeneratedKeys[0].Path)
	}
}

func TestRun_AccountAddNonInteractive_DuplicateKeyMessage(t *testing.T) {
	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID:                   "personal",
		DisplayName:          "Personal",
		PublicKeyFingerprint: "SHA256:shared",
	})
	ssh := fakes.NewFakeSSHClient()
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	ssh.Keys[keyPath] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:shared"}
	deps, _, stderr := addDeps(t, nil, repo, ssh, fakes.NewFakeSSHAgentAdapter())

	code := cli.Run([]string{
		"account", "add", "--non-interactive",
		"--name", "Lean Tech",
		"--username", "cristhiandelgado-work",
		"--commit-name", "Cristhian Delgado",
		"--commit-email", "cristhian@leantech.com",
		"--key", keyPath,
	}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	want := "is already used by the account \"Personal\""
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), want)
	}
	if !strings.Contains(stderr.String(), "GitHub does not allow the same public key on two accounts.") {
		t.Fatalf("missing explanation line: %s", stderr.String())
	}
}

// TestRun_AccountAddNonInteractive_VerificationSucceeds is RF-13 closing
// the Sprint 2 flow: after the key is resolved, Git Rodolfo tests it
// against GitHub and reports the result.
func TestRun_AccountAddNonInteractive_VerificationSucceeds(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	ssh := fakes.NewFakeSSHClient()
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	ssh.Keys[keyPath] = fakes.FakeKey{
		Valid:       true,
		Fingerprint: "SHA256:aaa",
		AuthResult:  domain.AuthResult{Success: true, Username: "cristhiandelgado-work", RawOutput: "Hi cristhiandelgado-work!"},
	}
	deps, stdout, stderr := addDeps(t, nil, repo, ssh, fakes.NewFakeSSHAgentAdapter())

	code := cli.Run([]string{
		"account", "add", "--non-interactive",
		"--name", "Lean Tech",
		"--username", "cristhiandelgado-work",
		"--commit-name", "Cristhian Delgado",
		"--commit-email", "cristhian@leantech.com",
		"--key", keyPath,
	}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	for _, want := range []string{"Testing authentication...", "✓ SSH key accepted", "✓ GitHub account detected: cristhiandelgado-work"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output missing %q:\n%s", want, stdout.String())
		}
	}
	saved, _ := repo.FindByID("lean-tech")
	if saved.AuthenticationStatus != domain.AuthenticationStatusVerified {
		t.Fatalf("status = %q, want verified", saved.AuthenticationStatus)
	}
}

// TestRun_AccountAddNonInteractive_VerificationMismatch is §21's "the key
// authenticates as another user".
func TestRun_AccountAddNonInteractive_VerificationMismatch(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	ssh := fakes.NewFakeSSHClient()
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	ssh.Keys[keyPath] = fakes.FakeKey{
		Valid:       true,
		Fingerprint: "SHA256:aaa",
		AuthResult:  domain.AuthResult{Success: true, Username: "cristhiandelgado", RawOutput: "Hi cristhiandelgado!"},
	}
	deps, stdout, stderr := addDeps(t, nil, repo, ssh, fakes.NewFakeSSHAgentAdapter())

	code := cli.Run([]string{
		"account", "add", "--non-interactive",
		"--name", "Lean Tech",
		"--username", "cristhiandelgado-work",
		"--commit-name", "Cristhian Delgado",
		"--commit-email", "cristhian@leantech.com",
		"--key", keyPath,
	}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `authenticated successfully, but as "cristhiandelgado"`) {
		t.Fatalf("missing mismatch message: %s", stdout.String())
	}
	saved, _ := repo.FindByID("lean-tech")
	if saved.AuthenticationStatus != domain.AuthenticationStatusFailed {
		t.Fatalf("a mismatch must not be recorded as verified, got %q", saved.AuthenticationStatus)
	}
}

func TestRun_AccountAddNonInteractive_RegisterWithGH_Success(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	ssh := fakes.NewFakeSSHClient()
	ssh.GenerateFingerprint = "SHA256:aaa"
	deps, stdout, stderr := addDeps(t, nil, repo, ssh, fakes.NewFakeSSHAgentAdapter())
	provider := deps.Provider.(*fakes.FakeProviderClient)
	provider.CLIUsername = "cristhiandelgado-work"

	code := cli.Run([]string{
		"account", "add", "--non-interactive",
		"--name", "Lean Tech",
		"--username", "cristhiandelgado-work",
		"--commit-name", "Cristhian Delgado",
		"--commit-email", "cristhian@leantech.com",
		"--generate-key",
		"--register-key-with-gh",
	}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "✓ Public key registered on GitHub.") {
		t.Fatalf("missing registration confirmation: %s", stdout.String())
	}
	if len(provider.RegisteredKeys) != 1 {
		t.Fatalf("expected one registered key, got %d", len(provider.RegisteredKeys))
	}
}

func TestRun_AccountAddNonInteractive_RegisterWithGH_WrongSession(t *testing.T) {
	repo := fakes.NewFakeAccountRepository()
	ssh := fakes.NewFakeSSHClient()
	ssh.GenerateFingerprint = "SHA256:aaa"
	deps, _, stderr := addDeps(t, nil, repo, ssh, fakes.NewFakeSSHAgentAdapter())
	provider := deps.Provider.(*fakes.FakeProviderClient)
	provider.CLIUsername = "cristhiandelgado" // different from the account below

	code := cli.Run([]string{
		"account", "add", "--non-interactive",
		"--name", "Lean Tech",
		"--username", "cristhiandelgado-work",
		"--commit-name", "Cristhian Delgado",
		"--commit-email", "cristhian@leantech.com",
		"--generate-key",
		"--register-key-with-gh",
	}, deps)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), `GitHub CLI is authenticated as "cristhiandelgado"`) {
		t.Fatalf("missing mismatch message: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "gh auth switch") {
		t.Fatalf("missing suggested command: %s", stderr.String())
	}
	if len(provider.RegisteredKeys) != 0 {
		t.Fatal("key must not be registered when gh's session doesn't match")
	}
	// The account itself must still be saved — registration failing is
	// informational, not a rollback trigger.
	if saved, _ := repo.FindByID("lean-tech"); saved == nil {
		t.Fatal("account should still be saved despite the registration mismatch")
	}
}

// TestRun_AccountAddInteractive_ConfigureLater drives the full interactive
// wizard through a pipe. Since a pipe is never a TTY, Select falls back to
// its numbered-list mode, which makes the whole flow scriptable and
// deterministic without a real terminal.
func TestRun_AccountAddInteractive_ConfigureLater(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	repo := fakes.NewFakeAccountRepository()
	deps, stdout, stderr := addDeps(t, r, repo, fakes.NewFakeSSHClient(), fakes.NewFakeSSHAgentAdapter())

	go func() {
		defer w.Close()
		w.WriteString("Lean Tech\n")
		w.WriteString("cristhiandelgado-work\n")
		w.WriteString("Cristhian Delgado\n")
		w.WriteString("cristhian@leantech.com\n")
		w.WriteString("3\n") // "Configure later"
	}()

	code := cli.Run([]string{"account", "add"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `Account "Lean Tech" saved.`) {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
	if saved, _ := repo.FindByID("lean-tech"); saved == nil {
		t.Fatal("account not persisted")
	}
}

func TestRun_AccountAddInteractive_GenerateKeyNoPassphrase(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	repo := fakes.NewFakeAccountRepository()
	ssh := fakes.NewFakeSSHClient()
	ssh.GenerateFingerprint = "SHA256:generated"
	deps, stdout, stderr := addDeps(t, r, repo, ssh, fakes.NewFakeSSHAgentAdapter())

	go func() {
		defer w.Close()
		w.WriteString("Lean Tech\n")
		w.WriteString("cristhiandelgado-work\n")
		w.WriteString("Cristhian Delgado\n")
		w.WriteString("cristhian@leantech.com\n")
		w.WriteString("1\n") // "Generate a new key"
		w.WriteString("\n")  // accept default filename
		w.WriteString("n\n") // no passphrase
	}()

	code := cli.Run([]string{"account", "add"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	saved, _ := repo.FindByID("lean-tech")
	if saved == nil || saved.PublicKeyFingerprint != "SHA256:generated" {
		t.Fatalf("unexpected saved account: %+v", saved)
	}
	if len(ssh.GeneratedKeys) != 1 || ssh.GeneratedKeys[0].WithPassphrase {
		t.Fatalf("unexpected GenerateKey calls: %+v", ssh.GeneratedKeys)
	}
}

// TestRun_AccountAddInteractive_ScannedKeyAvailable covers RF-40: a key
// found scanning SSHDir is offered in the selector, and picking it
// associates the account directly — no "Path to the private key" prompt.
func TestRun_AccountAddInteractive_ScannedKeyAvailable(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	repo := fakes.NewFakeAccountRepository()
	ssh := fakes.NewFakeSSHClient()
	deps, stdout, stderr := addDeps(t, r, repo, ssh, fakes.NewFakeSSHAgentAdapter())

	keyPath := filepath.Join(deps.SSHDir, "id_ed25519")
	ssh.Keys[keyPath] = fakes.FakeKey{Valid: true, Fingerprint: "SHA256:scanned"}
	ssh.ScanResults = map[string][]domain.SSHKeyCandidate{
		deps.SSHDir: {{PrivateKeyPath: keyPath, PublicKeyPath: keyPath + ".pub", Fingerprint: "SHA256:scanned"}},
	}

	go func() {
		defer w.Close()
		w.WriteString("Lean Tech\n")
		w.WriteString("cristhiandelgado-work\n")
		w.WriteString("Cristhian Delgado\n")
		w.WriteString("cristhian@leantech.com\n")
		w.WriteString("1\n") // the scanned key
	}()

	code := cli.Run([]string{"account", "add"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), "Path to the private key") {
		t.Fatalf("should not have prompted for a path already known from the scan: %s", stdout.String())
	}
	saved, _ := repo.FindByID("lean-tech")
	if saved == nil || saved.PrivateKeyPath != keyPath || saved.PublicKeyFingerprint != "SHA256:scanned" {
		t.Fatalf("unexpected saved account: %+v", saved)
	}
}

// TestRun_AccountAddInteractive_ScannedKeyInUse_Reprompts covers RF-41: a
// scanned key already claimed by another account is shown, not hidden,
// and selecting it re-prompts instead of letting it through.
func TestRun_AccountAddInteractive_ScannedKeyInUse_Reprompts(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	repo := fakes.NewFakeAccountRepository(domain.Account{
		ID:                   "lean-tech",
		DisplayName:          "Lean Tech",
		PublicKeyFingerprint: "SHA256:taken",
	})
	ssh := fakes.NewFakeSSHClient()
	deps, stdout, stderr := addDeps(t, r, repo, ssh, fakes.NewFakeSSHAgentAdapter())

	keyPath := filepath.Join(deps.SSHDir, "id_ed25519_work")
	ssh.ScanResults = map[string][]domain.SSHKeyCandidate{
		deps.SSHDir: {{PrivateKeyPath: keyPath, PublicKeyPath: keyPath + ".pub", Fingerprint: "SHA256:taken"}},
	}

	go func() {
		defer w.Close()
		w.WriteString("New Account\n")
		w.WriteString("someone\n")
		w.WriteString("Someone\n")
		w.WriteString("someone@example.com\n")
		w.WriteString("1\n") // the in-use scanned key — should re-prompt
		w.WriteString("4\n") // now "Configure later"
	}()

	code := cli.Run([]string{"account", "add"}, deps)

	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %s, stdout = %s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `already used by account "Lean Tech"`) {
		t.Fatalf("expected an in-use warning, got: %s", stdout.String())
	}
	saved, _ := repo.FindByID("new-account")
	if saved == nil || saved.PrivateKeyPath != "" {
		t.Fatalf("unexpected saved account: %+v", saved)
	}
}
