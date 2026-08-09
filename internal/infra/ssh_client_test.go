package infra

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lean-tech/git-rodolfo/internal/app"
)

func requireSSHTools(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not found on PATH")
	}
}

func TestSSHClient_GenerateValidateFingerprint(t *testing.T) {
	requireSSHTools(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "id_test")
	c := NewSSHClient()

	if err := c.GenerateKey(path, "test@example.com", false, false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := c.ValidateKey(path); err != nil {
		t.Fatalf("validate: %v", err)
	}
	fp, err := c.PublicKeyFingerprint(path)
	if err != nil {
		t.Fatalf("fingerprint: %v", err)
	}
	if !strings.HasPrefix(fp, "SHA256:") {
		t.Fatalf("unexpected fingerprint format: %q", fp)
	}
	hasPass, err := c.KeyHasPassphrase(path)
	if err != nil {
		t.Fatalf("KeyHasPassphrase: %v", err)
	}
	if hasPass {
		t.Fatal("expected no passphrase on a -N \"\" key")
	}

	content, err := c.PublicKeyContent(path)
	if err != nil {
		t.Fatalf("PublicKeyContent: %v", err)
	}
	if !strings.HasPrefix(content, "ssh-ed25519 ") {
		t.Fatalf("unexpected public key content: %q", content)
	}
}

// TestSSHClient_PublicKeyContent_DerivesWithoutPubFile covers the fallback
// path: if the .pub file is missing, PublicKeyContent must still work by
// deriving it from the private key directly (no passphrase needed, per
// the package doc).
func TestSSHClient_PublicKeyContent_DerivesWithoutPubFile(t *testing.T) {
	requireSSHTools(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "id_test")
	c := NewSSHClient()

	if err := c.GenerateKey(path, "test@example.com", false, false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := os.Remove(path + ".pub"); err != nil {
		t.Fatalf("remove .pub fixture: %v", err)
	}

	content, err := c.PublicKeyContent(path)
	if err != nil {
		t.Fatalf("PublicKeyContent: %v", err)
	}
	if !strings.HasPrefix(content, "ssh-ed25519 ") {
		t.Fatalf("unexpected public key content: %q", content)
	}
}

func TestSSHClient_KeyFilePermissions(t *testing.T) {
	requireSSHTools(t)
	if runtime.GOOS == "windows" {
		// os.Stat's mode bits are synthetic on Windows (always ~0666 for a
		// writable file) — they don't reflect real ACL security at all.
		// KeyFilePermissions gets replaced with an ACL-aware equivalent in
		// PR2 (RF-46); nothing meaningful to assert here until then.
		t.Skip("POSIX permission bits are not meaningful on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "id_test")
	c := NewSSHClient()

	if err := c.GenerateKey(path, "test@example.com", false, false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	perm, err := c.KeyFilePermissions(path)
	if err != nil {
		t.Fatalf("KeyFilePermissions: %v", err)
	}
	// ssh-keygen creates private keys with 600 by default.
	if perm&0o077 != 0 {
		t.Fatalf("expected a freshly generated key to have secure permissions, got %04o", perm)
	}

	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	perm, err = c.KeyFilePermissions(path)
	if err != nil {
		t.Fatalf("KeyFilePermissions: %v", err)
	}
	if perm != 0o644 {
		t.Fatalf("got %04o, want 0644", perm)
	}
}

func TestSSHClient_DeleteKey(t *testing.T) {
	requireSSHTools(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "id_test")
	c := NewSSHClient()

	if err := c.GenerateKey(path, "test@example.com", false, false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := c.DeleteKey(path); err != nil {
		t.Fatalf("DeleteKey: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("private key still exists after DeleteKey: err=%v", err)
	}
	if _, err := os.Stat(path + ".pub"); !os.IsNotExist(err) {
		t.Fatalf("public key still exists after DeleteKey: err=%v", err)
	}
}

func TestSSHClient_GenerateKey_RefusesToOverwriteWithoutFlag(t *testing.T) {
	requireSSHTools(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "id_test")
	c := NewSSHClient()

	if err := c.GenerateKey(path, "test@example.com", false, false); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := c.GenerateKey(path, "test@example.com", false, false); !errors.Is(err, app.ErrKeyFileExists) {
		t.Fatalf("expected ErrKeyFileExists, got %v", err)
	}
	if err := c.GenerateKey(path, "test@example.com", false, true); err != nil {
		t.Fatalf("expected overwrite=true to succeed, got %v", err)
	}
}

// TestSSHClient_PassphraseProtectedKey seeds a key with ssh-keygen directly
// (bypassing our GenerateKey, which never handles a passphrase itself) to
// verify KeyHasPassphrase detects it, and that ValidateKey/
// PublicKeyFingerprint work on it WITHOUT needing the passphrase — the
// whole point of relying on the OpenSSH key format's cleartext public part.
func TestSSHClient_PassphraseProtectedKey(t *testing.T) {
	requireSSHTools(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "id_protected")

	seed := exec.Command("ssh-keygen", "-t", "ed25519", "-C", "x", "-f", path, "-N", "s3cret-passphrase")
	if out, err := seed.CombinedOutput(); err != nil {
		t.Fatalf("seed passphrase-protected key: %v: %s", err, out)
	}

	c := NewSSHClient()
	hasPass, err := c.KeyHasPassphrase(path)
	if err != nil {
		t.Fatalf("KeyHasPassphrase: %v", err)
	}
	if !hasPass {
		t.Fatal("expected key to be detected as passphrase-protected")
	}
	if err := c.ValidateKey(path); err != nil {
		t.Fatalf("validate should not require the passphrase: %v", err)
	}
	if _, err := c.PublicKeyFingerprint(path); err != nil {
		t.Fatalf("fingerprint should not require the passphrase: %v", err)
	}
}

func TestSSHClient_ValidateKey_RejectsGarbage(t *testing.T) {
	requireSSHTools(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "not-a-key")
	if err := os.WriteFile(path, []byte("this is not a key"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := NewSSHClient()
	if err := c.ValidateKey(path); err == nil {
		t.Fatal("expected an error validating a non-key file")
	}
}

func TestParseAuthResult(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantOK   bool
		wantUser string
	}{
		{
			name:     "success despite exit code 1",
			output:   "Hi cristhiandelgado-work! You've successfully authenticated, but GitHub does not provide shell access.\n",
			wantOK:   true,
			wantUser: "cristhiandelgado-work",
		},
		{
			name:   "permission denied",
			output: "git@github.com: Permission denied (publickey).\n",
			wantOK: false,
		},
		{
			name:   "empty output",
			output: "",
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAuthResult(tt.output)
			if got.Success != tt.wantOK {
				t.Fatalf("Success = %v, want %v", got.Success, tt.wantOK)
			}
			if got.Username != tt.wantUser {
				t.Fatalf("Username = %q, want %q", got.Username, tt.wantUser)
			}
			if got.RawOutput != tt.output {
				t.Fatalf("RawOutput not preserved")
			}
		})
	}
}

// TestSSHClient_ScanKeys covers RF-40/RF-42: only well-formed pairs (a
// private key with a matching, valid .pub) come back, orphaned files on
// either side are skipped, and a missing directory yields no candidates
// rather than an error.
func TestSSHClient_ScanKeys(t *testing.T) {
	requireSSHTools(t)
	dir := t.TempDir()
	c := NewSSHClient()

	keyA := filepath.Join(dir, "id_ed25519_a")
	if err := c.GenerateKey(keyA, "a@example.com", false, false); err != nil {
		t.Fatalf("generate a: %v", err)
	}
	keyB := filepath.Join(dir, "id_ed25519_b")
	if err := c.GenerateKey(keyB, "b@example.com", false, false); err != nil {
		t.Fatalf("generate b: %v", err)
	}

	// Orphaned .pub with no private key next to it.
	if err := os.WriteFile(filepath.Join(dir, "orphan.pub"), []byte("ssh-ed25519 AAAA orphan\n"), 0o644); err != nil {
		t.Fatalf("write orphan.pub: %v", err)
	}
	// Private key with no .pub at all — not a candidate (needs both files).
	if err := os.WriteFile(filepath.Join(dir, "no_pub"), []byte("not a real key"), 0o600); err != nil {
		t.Fatalf("write no_pub: %v", err)
	}
	// A ".pub" sitting next to a file that isn't a valid private key.
	if err := os.WriteFile(filepath.Join(dir, "corrupt"), []byte("garbage"), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "corrupt.pub"), []byte("ssh-ed25519 AAAA corrupt\n"), 0o644); err != nil {
		t.Fatalf("write corrupt.pub: %v", err)
	}

	got, err := c.ScanKeys(dir)
	if err != nil {
		t.Fatalf("ScanKeys: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d candidates, want 2: %+v", len(got), got)
	}
	for _, cand := range got {
		if cand.PrivateKeyPath != keyA && cand.PrivateKeyPath != keyB {
			t.Fatalf("unexpected candidate %q", cand.PrivateKeyPath)
		}
		if cand.PublicKeyPath != cand.PrivateKeyPath+".pub" {
			t.Fatalf("PublicKeyPath = %q, want %q.pub", cand.PublicKeyPath, cand.PrivateKeyPath)
		}
		if !strings.HasPrefix(cand.Fingerprint, "SHA256:") {
			t.Fatalf("unexpected fingerprint format: %q", cand.Fingerprint)
		}
	}
}

func TestSSHClient_ScanKeys_MissingDir(t *testing.T) {
	c := NewSSHClient()
	got, err := c.ScanKeys(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("ScanKeys on missing dir: %v", err)
	}
	if got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}
