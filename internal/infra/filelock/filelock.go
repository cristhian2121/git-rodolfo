// Package filelock provides an exclusive, cross-process advisory lock used
// to serialize writes to config.json (PRD RNF-06). Acquire/Unlock are
// implemented per-OS (filelock_unix.go, filelock_windows.go) since neither
// platform's underlying primitive (flock(2), LockFileEx) is portable.
package filelock

import "os"

// Lock is a held exclusive lock on a sidecar file. Release it with Unlock.
type Lock struct {
	file *os.File
}
