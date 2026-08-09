package infra

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lean-tech/git-rodolfo/internal/infra/atomicfile"
)

// SelfUpdater replaces the currently running git-rodolfo executable with a
// new binary's bytes (RF §26's "update" command). It reuses atomicfile's
// temp-file-plus-rename write: the temp file is created in the same
// directory as the target, so the final rename is atomic and a process
// killed mid-update never leaves a half-written binary in place — a
// currently-running copy of the old binary keeps working off its already
// -open file handle until it exits, same as any other in-place binary
// replacement (how package managers and other self-updaters do this too).
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

	if err := atomicfile.Write(exe, newBinary, 0o755); err != nil {
		return fmt.Errorf("replace %s: %w", exe, err)
	}
	return nil
}
