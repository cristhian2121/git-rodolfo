package infra

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func requireSSHAgentTools(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ssh-add"); err != nil {
		t.Skip("ssh-add not found on PATH")
	}
}

// requireWindows skips tests that only make sense on a real Windows
// machine (e.g. exercising icacls-based ACL checks) — mirrors
// requireSSHAgentTools/requireSSHTools's runtime-skip style rather than a
// //go:build tag, consistent with the rest of this codebase.
func requireWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("this test only runs on Windows")
	}
}

func TestSSHAgentAdapter_IsRunning(t *testing.T) {
	requireSSHAgentTools(t)
	a := NewSSHAgentAdapter()
	// Read-only: just exercises the real agent on this machine (or its
	// absence) without asserting a specific value either way.
	_ = a.IsRunning()
}

func TestSSHAgentAdapter_IsKeyLoaded_UnloadedKey(t *testing.T) {
	requireSSHAgentTools(t)
	if !NewSSHAgentAdapter().IsRunning() {
		t.Skip("no ssh-agent reachable")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "id_never_loaded")
	if err := NewSSHClient().GenerateKey(path, "test@example.com", false, false); err != nil {
		t.Fatalf("generate: %v", err)
	}

	loaded, err := NewSSHAgentAdapter().IsKeyLoaded(path)
	if err != nil {
		t.Fatalf("IsKeyLoaded: %v", err)
	}
	if loaded {
		t.Fatal("a freshly generated key should not already be in the agent")
	}
}

// TestSSHAgentAdapter_AddKey mutates the real, shared ssh-agent on the
// machine running the test (loading a throwaway key into it), so it only
// runs when explicitly opted into — never as a side effect of a plain
// `go test ./...`.
func TestSSHAgentAdapter_AddKey(t *testing.T) {
	requireSSHAgentTools(t)
	if os.Getenv("GIT_RODOLFO_TEST_SSH_AGENT") != "1" {
		t.Skip("set GIT_RODOLFO_TEST_SSH_AGENT=1 to run a test that loads a key into the real ssh-agent")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "id_add_test")
	if err := NewSSHClient().GenerateKey(path, "test@example.com", false, false); err != nil {
		t.Fatalf("generate: %v", err)
	}

	agent := NewSSHAgentAdapter()
	if err := agent.AddKey(path, false); err != nil {
		t.Fatalf("AddKey: %v", err)
	}
	t.Cleanup(func() { exec.Command("ssh-add", "-d", path).Run() })

	loaded, err := agent.IsKeyLoaded(path)
	if err != nil {
		t.Fatalf("IsKeyLoaded: %v", err)
	}
	if !loaded {
		t.Fatal("expected key to be loaded after AddKey")
	}
}
