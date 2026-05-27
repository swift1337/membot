package indexer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireLockRelease(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "index.lock")
	lock, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}
	if err := lock.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock file removed, stat err = %v", err)
	}
}

func TestAcquireLockHeld(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "index.lock")
	first, err := AcquireLock(lockPath)
	if err != nil {
		t.Fatalf("AcquireLock() first error = %v", err)
	}
	t.Cleanup(func() {
		_ = first.Release()
	})

	_, err = AcquireLock(lockPath)
	if err == nil {
		t.Fatal("expected second AcquireLock() to fail")
	}
	if !errors.Is(err, ErrLockHeld) {
		t.Fatalf("AcquireLock() error = %v, want ErrLockHeld", err)
	}
}
