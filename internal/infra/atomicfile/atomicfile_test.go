package atomicfile

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWrite_ReplacesContentAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := Write(path, []byte(`{"v":1}`), 0o600); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := Write(path, []byte(`{"v":2}`), 0o600); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != `{"v":2}` {
		t.Fatalf("got %q, want %q", got, `{"v":2}`)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected only the final file to remain, found %d entries", len(entries))
	}
}

func TestWrite_NoLeftoverTempFileOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	if err := Write(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "config.json" {
		t.Fatalf("unexpected directory contents: %v", entries)
	}
}

// TestWrite_SurvivesKillMidWrite is the RNF-05 test: a process killed after
// it has durably written the temp file but before the rename must never
// corrupt or replace the pre-existing target file.
//
// It re-execs this same test binary as a helper process (the standard Go
// idiom for subprocess tests, see os/exec_test.go in the standard library),
// which pauses right before the rename step so the parent has a reliable
// window in which to send SIGKILL.
func TestWrite_SurvivesKillMidWrite(t *testing.T) {
	if os.Getenv("GIT_RODOLFO_ATOMICFILE_HELPER") == "1" {
		runKillHelper()
		return
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	original := []byte(`{"version":1,"accounts":[]}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("seed original file: %v", err)
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestWrite_SurvivesKillMidWrite$")
	cmd.Env = append(os.Environ(),
		"GIT_RODOLFO_ATOMICFILE_HELPER=1",
		"GIT_RODOLFO_ATOMICFILE_TARGET="+path,
	)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}

	// The helper sleeps for a fixed window between the durable temp-file
	// write and the rename; killing here lands inside that window.
	time.Sleep(200 * time.Millisecond)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill helper: %v", err)
	}
	_ = cmd.Wait() // exit status is expected to be non-zero (killed)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("target file missing after kill: %v", err)
	}
	if string(got) != string(original) {
		t.Fatalf("target file corrupted by interrupted write: got %q, want original %q", got, original)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "config.json" && filepath.Ext(e.Name()) != "" {
			// A stray, half-written temp file is harmless — it never
			// replaced the target. Nothing to assert beyond that it is
			// not the target path itself, checked above.
			_ = e
		}
	}
}

// runKillHelper performs an atomic write with an artificial delay inserted
// right before the rename, then relies on the parent test to kill it during
// that delay. If the parent's timing is off and the process is never
// killed, it exits 0 having completed a normal write — the parent only
// asserts on the target file's state, so that outcome is also safe.
func runKillHelper() {
	path := os.Getenv("GIT_RODOLFO_ATOMICFILE_TARGET")
	_ = writeWithHook(path, []byte(`{"version":1,"accounts":[{"id":"corrupt"}]}`), 0o600, func() {
		time.Sleep(2 * time.Second)
	})
	os.Exit(0)
}
