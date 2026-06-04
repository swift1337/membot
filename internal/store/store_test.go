package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRemoveDatabaseFilesDeletesDB(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "membot.db")
	st, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if _, err := st.DB().ExecContext(ctx, `INSERT INTO sources(id, kind, root_path) VALUES (1, 'cursor', '/tmp')`); err != nil {
		t.Fatalf("insert source: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := RemoveDatabaseFiles(dbPath); err != nil {
		t.Fatalf("RemoveDatabaseFiles() error = %v", err)
	}

	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, stat err = %v", dbPath, err)
	}
}

func TestOpenLimitsConnectionPool(t *testing.T) {
	t.Parallel()

	st, err := Open(context.Background(), filepath.Join(t.TempDir(), "membot.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		_ = st.Close()
	}()

	if got := st.DB().Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", got)
	}
}
