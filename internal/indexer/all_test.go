package indexer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/swift1337/membot/internal/store"
)

func TestFoundSourceNames(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cursorRoot := filepath.Join(root, "cursor")
	claudeRoot := filepath.Join(root, "claude")
	codexRoot := filepath.Join(root, "codex")
	for _, dir := range []string{
		cursorRoot,
		filepath.Join(claudeRoot, "projects"),
		filepath.Join(codexRoot, "sessions"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", dir, err)
		}
	}

	got := FoundSourceNames(AllOptions{
		Cursor: CursorOptions{Root: cursorRoot},
		Claude: ClaudeOptions{Root: claudeRoot},
		Codex:  CodexOptions{Root: codexRoot},
	})
	want := []string{"Cursor", "Claude", "Codex"}
	if len(got) != len(want) {
		t.Fatalf("FoundSourceNames() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("source %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestIndexAllSkipsMissingSources(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "membot.db"))
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})

	root := t.TempDir()
	result, err := IndexAll(ctx, st, AllOptions{
		Cursor: CursorOptions{Root: filepath.Join(root, "missing-cursor")},
		Claude: ClaudeOptions{Root: filepath.Join(root, "missing-claude")},
		Codex:  CodexOptions{Root: filepath.Join(root, "missing-codex")},
	})
	if err != nil {
		t.Fatalf("IndexAll() error = %v", err)
	}
	if len(result.Indexers) != 0 {
		t.Fatalf("IndexAll() indexers = %#v, want none", result.Indexers)
	}
}
