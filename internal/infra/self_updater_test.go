package infra

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestSelfUpdater_Replace_ReplacesRunningExecutable is the real end-to-end
// check for SelfUpdater: os.Executable() can't be faked from within this
// process (it always resolves to whatever binary is actually running), so
// this copies the compiled test binary to a scratch path and re-execs it
// as a subprocess with a helper flag — same idiom as
// atomicfile_test.go's TestWrite_SurvivesKillMidWrite. Inside that
// subprocess, its own os.Executable() *is* the scratch copy, so calling
// Replace there exercises the exact resolution + atomic-write path a real
// "git-rodolfo update" run does, including overwriting a binary while it
// is the one currently executing.
func TestSelfUpdater_Replace_ReplacesRunningExecutable(t *testing.T) {
	if os.Getenv("GIT_RODOLFO_SELFUPDATER_HELPER") == "1" {
		runSelfUpdaterHelper()
		return
	}

	self, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable: %v", err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatalf("read the current test binary: %v", err)
	}

	dir := t.TempDir()
	copyName := "fake-git-rodolfo"
	if runtime.GOOS == "windows" {
		// exec.Command on Windows treats a file without a recognized
		// executable extension as nonexistent, even given a full path.
		copyName += ".exe"
	}
	copyPath := filepath.Join(dir, copyName)
	if err := os.WriteFile(copyPath, data, 0o755); err != nil {
		t.Fatalf("write scratch copy: %v", err)
	}

	cmd := exec.Command(copyPath, "-test.run=^TestSelfUpdater_Replace_ReplacesRunningExecutable$")
	cmd.Env = append(os.Environ(), "GIT_RODOLFO_SELFUPDATER_HELPER=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("helper subprocess failed: %v: %s", err, out)
	}

	got, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if string(got) != selfUpdaterTestMarker {
		t.Fatalf("binary was not replaced as expected; got %d bytes, want marker content", len(got))
	}

	if runtime.GOOS != "windows" {
		// Windows has no POSIX execute bit — os.Stat there synthesizes
		// mode bits from the read-only attribute alone, never 0100
		// regardless of the file's actual (extension-based) executability.
		info, err := os.Stat(copyPath)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if info.Mode().Perm()&0o100 == 0 {
			t.Fatalf("expected the replaced file to remain executable, got mode %v", info.Mode())
		}
	}

	if runtime.GOOS == "windows" {
		// RF-47: the original binary should have been renamed aside
		// rather than deleted, since CleanupOldBinary is what removes it
		// (normally at the next startup, once nothing has it open).
		if _, err := os.Stat(copyPath + ".old"); err != nil {
			t.Fatalf("expected %s.old to remain for later cleanup: %v", copyPath, err)
		}
	}
}

const selfUpdaterTestMarker = "GIT_RODOLFO_SELFUPDATER_TEST_MARKER"

// runSelfUpdaterHelper is the subprocess side of the test above: it calls
// the real SelfUpdater.Replace on its own running executable, then exits.
func runSelfUpdaterHelper() {
	if err := NewSelfUpdater().Replace([]byte(selfUpdaterTestMarker)); err != nil {
		fmt.Fprintf(os.Stderr, "Replace failed: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// cleanupOldBinary takes the executable path as a parameter (see
// self_updater.go), so — unlike Replace — it's directly testable without
// the os.Executable()-dependent subprocess dance.
func TestCleanupOldBinary(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "git-rodolfo.exe")

	if err := cleanupOldBinary(exe); err != nil {
		t.Fatalf("cleanupOldBinary with nothing to clean up: %v", err)
	}

	if err := os.WriteFile(exe+".old", []byte("stale"), 0o755); err != nil {
		t.Fatalf("write stale .old: %v", err)
	}
	if err := cleanupOldBinary(exe); err != nil {
		t.Fatalf("cleanupOldBinary: %v", err)
	}
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Fatalf("expected %s.old to be removed, stat err = %v", exe, err)
	}
}
