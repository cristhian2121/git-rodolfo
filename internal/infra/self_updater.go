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
	old := exe + ".old"
	// A stale .old can only be left by a previous update that never got
	// cleaned up (RF-47); remove it first so the rename below can't fail
	// on an existing destination.
	if err := os.Remove(old); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale %s: %w", old, err)
	}
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("rename running executable aside: %w", err)
	}
	if err := atomicfile.Write(exe, newBinary, 0o755); err != nil {
		// Best effort: put the running binary back so the update failing
		// doesn't also leave the user without a working executable.
		os.Rename(old, exe)
		return fmt.Errorf("replace %s: %w", exe, err)
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
func (s *SelfUpdater) CleanupOldBinary() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate the running executable: %w", err)
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
