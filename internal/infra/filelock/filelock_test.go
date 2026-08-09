package filelock

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquire_CreatesFileIfMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json.lock")
	lock, err := Acquire(path)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer lock.Unlock()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected the lock file to be created: %v", err)
	}
}

// TestAcquire_SerializesAcrossHolders is RNF-06's core guarantee: a second
// Acquire on the same path must block until the first Lock is released,
// even though both live in the same process (flock is scoped to open file
// descriptions, not processes, so this still exercises the real guarantee
// two separate processes would rely on).
func TestAcquire_SerializesAcrossHolders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json.lock")

	first, err := Acquire(path)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}

	acquired := make(chan struct{})
	go func() {
		second, err := Acquire(path)
		if err != nil {
			t.Errorf("second Acquire: %v", err)
			return
		}
		close(acquired)
		second.Unlock()
	}()

	select {
	case <-acquired:
		t.Fatal("second Acquire should have blocked while the first lock is held")
	case <-time.After(100 * time.Millisecond):
		// expected: still blocked
	}

	if err := first.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	select {
	case <-acquired:
		// expected: unblocked after Unlock
	case <-time.After(2 * time.Second):
		t.Fatal("second Acquire did not unblock after the first lock was released")
	}
}
