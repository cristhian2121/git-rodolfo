package infra

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/lean-tech/git-rodolfo/internal/infra/atomicfile"
)

// SelfUpdater replaces the currently running git-rodolfo executable with a
// new binary's bytes (RF §26's "update" command). On Unix it reuses
// atomicfile's temp-file-plus-rename write: the temp file is created in
// the same directory as the target, so the final rename is atomic and a
// process killed mid-update never leaves a half-written binary in place —
// a currently-running copy of the old binary keeps working off its
// already-open file handle until it exits, same as any other in-place
// binary replacement (how package managers and other self-updaters do
// this too).
//
// Windows can't do that: overwriting the running executable's own path
// fails there with "Access is denied" (Windows locks a running exe
// against this the way Unix's rename(2) never does), but it does allow
// renaming that same running file to a different name. So on Windows,
// Replace instead renames the current exe aside to "<exe>.old", writes
// the new binary at the original path, and leaves ".old" for
// CleanupOldBinary to remove once nothing is running from it anymore
// (RF-47) — normally the very next time git-rodolfo starts.
type SelfUpdater struct{}

// NewSelfUpdater builds a SelfUpdater.
func NewSelfUpdater() *SelfUpdater {
	return &SelfUpdater{}
}

func (s *SelfUpdater) Replace(newBinary []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate the running executable: %w", err)
	}
	// Homebrew (and similar) installs are often a symlink into a
	// versioned cellar path; resolve it so the real binary gets replaced
	// rather than leaving a symlink pointing at whatever was there before.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	if runtime.GOOS == "windows" {
		return windowsReplace(exe, newBinary)
	}

	if err := atomicfile.Write(exe, newBinary, 0o755); err != nil {
		return fmt.Errorf("replace %s: %w", exe, err)
	}
	return nil
}

func windowsReplace(exe string, newBinary []byte) error {
	// Write the new binary to a temp file *before* touching exe at all:
	// once exe gets renamed aside below, the only work left is two fast,
	// metadata-only renames — no slow data write happens while exe is
	// briefly missing. Doing it in the other order (rename aside, then
	// write) would leave a crash-sized window where a kill/power-loss
	// mid-write leaves nothing at all runnable at the executable's path.
	dir := filepath.Dir(exe)
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(exe)+"-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(newBinary); err != nil {
		tmp.Close()
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close new binary: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("chmod new binary: %w", err)
	}

	// os.Rename mirrors POSIX rename(2) semantics on Windows too — it
	// overwrites an existing destination — so a stale ".old" left by a
	// previous update that never got cleaned up doesn't need removing
	// first; this rename replaces it directly.
	old := exe + ".old"
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("rename running executable aside: %w", err)
	}
	if err := os.Rename(tmpPath, exe); err != nil {
		// Best effort: put the running binary back so the update failing
		// doesn't also leave the user without a working executable. If
		// even that fails, say so explicitly rather than only reporting
		// the original error and leaving the user with neither exe nor
		// an obvious clue that ".old" holds their working binary.
		if rollbackErr := os.Rename(old, exe); rollbackErr != nil {
			return fmt.Errorf("rename new binary into place: %w (restoring the original also failed: %v; it can be recovered from %s)", err, rollbackErr, old)
		}
		return fmt.Errorf("rename new binary into place: %w", err)
	}
	return nil
}

// CleanupOldBinary removes a stale "<exe>.old" left by a previous Windows
// update run (RF-47) — a no-op on other platforms and when there's
// nothing to clean up. Meant to be called once at startup: by then the
// process that was running from ".old" at update time has exited, so
// nothing still holds it open. Callers should treat a failure here as
// non-fatal (log and continue) rather than exiting — worst case, a stray
// ".old" file just sits there until the next startup tries again.
//
// Known limitation: this doesn't coordinate against an update actually
// in flight in another process — running "update" concurrently with any
// other git-rodolfo invocation on Windows is unsupported.
func (s *SelfUpdater) CleanupOldBinary() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate the running executable: %w", err)
	}
	// Replace resolves symlinks before deriving "<exe>.old" (see above);
	// this has to match, or a symlinked install would look for the stale
	// file at the wrong path and never find it.
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return cleanupOldBinary(exe)
}

// cleanupOldBinary takes the resolved executable path as a parameter,
// rather than resolving it itself, so it's testable without the
// os.Executable()-dependent subprocess dance Replace's tests need.
func cleanupOldBinary(exe string) error {
	if err := os.Remove(exe + ".old"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s.old: %w", exe, err)
	}
	return nil
}
