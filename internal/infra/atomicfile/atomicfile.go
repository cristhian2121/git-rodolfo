// Package atomicfile writes files so that a process interrupted mid-write
// (crash, kill -9, power loss) never leaves the target path holding partial
// content (PRD RNF-05).
package atomicfile

import (
	"os"
	"path/filepath"
)

// Write replaces path's content with data. It writes to a temporary file in
// the same directory, flushes it to disk, and only then renames it over
// path — rename is atomic on the same filesystem, so readers of path always
// see either the old content or the new one, never a partial write.
func Write(path string, data []byte, perm os.FileMode) error {
	return writeWithHook(path, data, perm, nil)
}

// writeWithHook is Write with an injection point exercised only by tests
// (see atomicfile_test.go) to pause between the durable temp-file write and
// the rename, simulating a process killed mid-write.
func writeWithHook(path string, data []byte, perm os.FileMode, beforeRename func()) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}

	if beforeRename != nil {
		beforeRename()
	}

	return os.Rename(tmpPath, path)
}
