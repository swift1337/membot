package cursor

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/swift1337/membot/internal/db"
)

func TestSourceFileMetadataUnchanged(t *testing.T) {
	t.Parallel()

	modTime := time.Unix(1_700_000_000, 0)
	info := stubFileInfo{size: 42, modTime: modTime}
	file := db.SourceFile{
		SizeBytes:     sql.NullInt64{Int64: 42, Valid: true},
		MtimeUnix:     sql.NullInt64{Int64: modTime.Unix(), Valid: true},
		ParserVersion: parserVersion,
	}

	if !sourceFileMetadataUnchanged(file, info) {
		t.Fatal("expected unchanged metadata to match")
	}

	file.ParserVersion = "cursor-jsonl-v4"
	if sourceFileMetadataUnchanged(file, info) {
		t.Fatal("expected parser version mismatch to require reindex")
	}
}

func TestIndexSkipsUnchangedTranscriptWithoutRehashing(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	root := t.TempDir()
	projectDir := filepath.Join(root, "Users-me-proj")
	transcriptsDir := filepath.Join(projectDir, "agent-transcripts")
	if err := os.MkdirAll(transcriptsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	transcriptPath := filepath.Join(transcriptsDir, "chat.jsonl")
	transcript := `{"role":"user","message":{"content":[{"type":"text","text":"hello"}]}}`
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "membot.db")
	st, err := openTestStore(ctx, dbPath)
	if err != nil {
		t.Fatalf("openTestStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})

	first, err := Index(ctx, st, Options{Root: root})
	if err != nil {
		t.Fatalf("Index() first error = %v", err)
	}
	if first.FilesIndexed != 1 {
		t.Fatalf("first FilesIndexed = %d, want 1", first.FilesIndexed)
	}

	second, err := Index(ctx, st, Options{Root: root})
	if err != nil {
		t.Fatalf("Index() second error = %v", err)
	}
	if second.FilesSkipped != 1 {
		t.Fatalf("second FilesSkipped = %d, want 1", second.FilesSkipped)
	}
	if second.FilesIndexed != 0 {
		t.Fatalf("second FilesIndexed = %d, want 0", second.FilesIndexed)
	}
}

type stubFileInfo struct {
	size    int64
	modTime time.Time
}

func (s stubFileInfo) Name() string       { return "stub" }
func (s stubFileInfo) Size() int64        { return s.size }
func (s stubFileInfo) Mode() os.FileMode  { return 0o644 }
func (s stubFileInfo) ModTime() time.Time { return s.modTime }
func (s stubFileInfo) IsDir() bool        { return false }
func (s stubFileInfo) Sys() any           { return nil }
