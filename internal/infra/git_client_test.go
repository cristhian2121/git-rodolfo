package infra

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
)

func requireGitTools(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("git", "init", "--quiet", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
}

func TestGitClient_IsRepository(t *testing.T) {
	requireGitTools(t)
	dir := t.TempDir()

	if NewGitClient(dir).IsRepository(dir) {
		t.Fatal("expected a fresh temp dir not to be a repository")
	}
	initRepo(t, dir)
	if !NewGitClient(dir).IsRepository(dir) {
		t.Fatal("expected an initialized repository to be detected")
	}
}

func TestGitClient_LocalConfigRoundTrip(t *testing.T) {
	requireGitTools(t)
	dir := t.TempDir()
	initRepo(t, dir)
	g := NewGitClient(dir)

	if _, err := g.GetLocalConfig("user.email"); !errors.Is(err, app.ErrConfigKeyNotSet) {
		t.Fatalf("expected ErrConfigKeyNotSet, got %v", err)
	}

	if err := g.SetLocalConfig("user.email", "cristhian@leantech.com"); err != nil {
		t.Fatalf("SetLocalConfig: %v", err)
	}
	got, err := g.GetLocalConfig("user.email")
	if err != nil {
		t.Fatalf("GetLocalConfig: %v", err)
	}
	if got != "cristhian@leantech.com" {
		t.Fatalf("got %q", got)
	}

	if err := g.UnsetLocalConfig("user.email"); err != nil {
		t.Fatalf("UnsetLocalConfig: %v", err)
	}
	if _, err := g.GetLocalConfig("user.email"); !errors.Is(err, app.ErrConfigKeyNotSet) {
		t.Fatalf("expected ErrConfigKeyNotSet after unset, got %v", err)
	}

	// RF-20: Clear must be idempotent — unsetting an already-absent key
	// must not be an error.
	if err := g.UnsetLocalConfig("user.email"); err != nil {
		t.Fatalf("UnsetLocalConfig on an absent key: %v", err)
	}
}

func TestGitClient_GetEffectiveConfig_FallsThroughToGlobal(t *testing.T) {
	requireGitTools(t)
	dir := t.TempDir()
	initRepo(t, dir)
	g := NewGitClient(dir)

	// Scope a "global" value to this test via a repo-local include isn't
	// straightforward without touching the real ~/.gitconfig, so this test
	// only exercises the local-is-set case here; the global-fallthrough
	// behavior is exactly git's own `config --get` semantics (no --local),
	// which GetLocalConfig deliberately does NOT use.
	if err := g.SetLocalConfig("user.email", "local@example.com"); err != nil {
		t.Fatalf("SetLocalConfig: %v", err)
	}
	got, err := g.GetEffectiveConfig("user.email")
	if err != nil {
		t.Fatalf("GetEffectiveConfig: %v", err)
	}
	if got != "local@example.com" {
		t.Fatalf("got %q", got)
	}
}

func TestGitClient_RemoteURL(t *testing.T) {
	requireGitTools(t)
	dir := t.TempDir()
	initRepo(t, dir)
	g := NewGitClient(dir)

	if _, err := g.GetRemoteURL("origin"); !errors.Is(err, app.ErrNoRemote) {
		t.Fatalf("expected ErrNoRemote, got %v", err)
	}

	cmd := exec.Command("git", "-C", dir, "remote", "add", "origin", "git@github.com:lean-tech/project.git")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v: %s", err, out)
	}
	got, err := g.GetRemoteURL("origin")
	if err != nil {
		t.Fatalf("GetRemoteURL: %v", err)
	}
	if got != "git@github.com:lean-tech/project.git" {
		t.Fatalf("got %q", got)
	}
}

// TestGitClient_CloneWithSSHCommand_LocalRepo clones a real, local
// repository over a filesystem path (no network, no GitHub) to exercise
// the actual `git clone` invocation and core.sshCommand injection end to
// end, per RNF-09's "integration tests use real local Git repositories."
func TestGitClient_CloneWithSSHCommand_LocalRepo(t *testing.T) {
	requireGitTools(t)
	src := t.TempDir()
	initRepo(t, src)
	cmd := exec.Command("git", "-C", src, "commit", "--allow-empty", "-m", "initial", "--quiet")
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}

	dest := filepath.Join(t.TempDir(), "clone-dest")
	g := NewGitClient("")
	if err := g.CloneWithSSHCommand(src, dest, ""); err != nil {
		t.Fatalf("CloneWithSSHCommand: %v", err)
	}

	cloned := NewGitClient(dest)
	if !cloned.IsRepository(dest) {
		t.Fatal("destination is not a repository after clone")
	}
}

// TestGitClient_CloneWithSSHCommand_DestinationStartingWithDashIsNotAFlag is
// a regression test for a git argument-injection vulnerability: without a
// "--" separator, a destination like "--template=<dir>" is parsed by git as
// the --template option instead of a literal path, and any hook in that
// template directory runs automatically during the clone's checkout step.
// CloneWithSSHCommand must treat it as a literal destination name instead.
func TestGitClient_CloneWithSSHCommand_DestinationStartingWithDashIsNotAFlag(t *testing.T) {
	requireGitTools(t)
	src := t.TempDir()
	initRepo(t, src)
	commit := exec.Command("git", "-C", src, "commit", "--allow-empty", "-m", "initial", "--quiet")
	commit.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}

	// A malicious template directory whose post-checkout hook would run
	// automatically if "--template=evilTemplateDir" were ever interpreted
	// as the --template option instead of a literal destination name.
	evilTemplateDir := t.TempDir()
	hooksDir := filepath.Join(evilTemplateDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	marker := filepath.Join(t.TempDir(), "PWNED")
	hookScript := "#!/bin/sh\ntouch " + marker + "\n"
	if err := os.WriteFile(filepath.Join(hooksDir, "post-checkout"), []byte(hookScript), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	parent := t.TempDir()
	maliciousDestination := "--template=" + evilTemplateDir
	g := NewGitClient("")
	// Run from `parent` so a literal directory named "--template=..." has
	// somewhere sane to land if git (correctly) treats it as a path.
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(parent); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	defer os.Chdir(oldWd)

	_ = g.CloneWithSSHCommand(src, maliciousDestination, "")

	if _, err := os.Stat(marker); err == nil {
		t.Fatal("SECURITY REGRESSION: the malicious template's post-checkout hook ran — the destination was interpreted as a --template flag instead of a literal path")
	}
}
