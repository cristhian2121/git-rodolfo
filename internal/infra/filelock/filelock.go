// Package filelock provides an exclusive, cross-process advisory lock used
// to serialize writes to config.json (PRD RNF-06). It targets macOS and
// Linux only (PRD RNF-02), so it can rely on flock(2) directly.
package filelock

import (
	"os"
	"syscall"
)

// Lock is a held exclusive lock on a sidecar file. Release it with Unlock.
type Lock struct {
	file *os.File
}

// Acquire blocks until it obtains an exclusive lock on path, creating the
// file if needed. path should be a dedicated lock file (e.g.
// "config.json.lock"), not the file being protected, so the atomic
// temp-file-plus-rename dance never has to happen under an open fd.
func Acquire(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &Lock{file: f}, nil
}

// Unlock releases the lock and closes the underlying file descriptor.
func (l *Lock) Unlock() error {
	if err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN); err != nil {
		l.file.Close()
		return err
	}
	return l.file.Close()
}
