//go:build windows

package filelock

import (
	"os"

	"golang.org/x/sys/windows"
)

// Acquire blocks until it obtains an exclusive lock on path (via
// LockFileEx), creating the file if needed — same contract as the Unix
// implementation (filelock_unix.go). Locking a single byte at offset 0 is
// enough: every Acquire call locks that same range, so two holders still
// conflict exactly like flock's whole-file lock does.
func Acquire(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	ol := new(windows.Overlapped)
	if err := windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, ol); err != nil {
		f.Close()
		return nil, err
	}
	return &Lock{file: f}, nil
}

// Unlock releases the lock and closes the underlying file descriptor.
func (l *Lock) Unlock() error {
	ol := new(windows.Overlapped)
	if err := windows.UnlockFileEx(windows.Handle(l.file.Fd()), 0, 1, 0, ol); err != nil {
		l.file.Close()
		return err
	}
	return l.file.Close()
}
