package claude

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/swift1337/membot/internal/db"
	"github.com/swift1337/membot/internal/store"
)

func TestParseLineExtractsClaudeMessage(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"type":"user","message":{"role":"user","content":"please inspect ` + "`internal/db/db.go`" + `"},"timestamp":"2026-05-29T15:39:12.847Z","cwd":"/Users/me/proj"}`)
	line, ok, err := parseLine(raw, 1)
	if err != nil {
		t.Fatalf("parseLine() error = %v", err)
	}
	if !ok {
		t.Fatal("expected conversation line")
	}
	if line.Role != "user" {
		t.Fatalf("line.Role = %q, want user", line.Role)
	}
	const wantTime = "2026-05-29T15:39:12Z"
	if line.CreatedAt != wantTime {
		t.Fatalf("line.CreatedAt = %q, want %q", line.CreatedAt, wantTime)
	}
	if line.Text() != "please inspect `internal/db/db.go`" {
		t.Fatalf("line.Text() = %q", line.Text())
	}
	if line.Cwd != "/Users/me/proj" {
		t.Fatalf("line.Cwd = %q", line.Cwd)
	}
}

func TestParseLineSkipsClaudeMetadata(t *testing.T) {
	t.Parallel()

	_, ok, err := parseLine([]byte(`{"type":"mode","mode":"normal","sessionId":"abc"}`), 1)
	if err != nil {
		t.Fatalf("parseLine() error = %v", err)
	}
	if ok {
		t.Fatal("metadata line should be skipped")
	}
}

func TestIndexClaudeTranscript(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	root := t.TempDir()
	projectDir := filepath.Join(root, "projects", "-Users-me-proj")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}

	transcriptPath := filepath.Join(projectDir, "session-1.jsonl")
	transcript := `{"type":"mode","mode":"normal","sessionId":"session-1"}
{"type":"user","message":{"role":"user","content":"Please inspect @src/main.go and ` + "`internal/db/db.go`" + `"},"uuid":"u1","timestamp":"2026-05-29T15:39:12.847Z","cwd":"/Users/me/proj","sessionId":"session-1"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"I'll check it."},{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}}]},"uuid":"a1","timestamp":"2026-05-29T15:39:13.000Z","cwd":"/Users/me/proj","sessionId":"session-1"}
`
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	st, err := openTestStore(ctx, filepath.Join(t.TempDir(), "membot.db"))
	if err != nil {
		t.Fatalf("openTestStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})

	result, err := Index(ctx, st, Options{Root: root})
	if err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	if result.ProjectsDiscovered != 1 || result.FilesDiscovered != 1 ||
		result.FilesIndexed != 1 || result.MessagesIndexed != 2 || result.ParseErrors != 0 {
		t.Fatalf("Index() result = %#v", result)
	}

	source, err := st.Queries().GetSourceByKindRoot(ctx, db.GetSourceByKindRootParams{
		Kind:     sourceKind,
		RootPath: root,
	})
	if err != nil {
		t.Fatalf("GetSourceByKindRoot() error = %v", err)
	}
	conversation, err := st.Queries().GetConversationByExternalID(ctx, db.GetConversationByExternalIDParams{
		SourceID:   source.ID,
		ExternalID: "session-1",
	})
	if err != nil {
		t.Fatalf("GetConversationByExternalID() error = %v", err)
	}
	if !conversation.Title.Valid || conversation.Title.String != "Please inspect @src/main.go and `internal/db/db.go`" {
		t.Fatalf("conversation title = %#v", conversation.Title)
	}

	messages, err := st.Queries().ListConversationMessages(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(messages))
	}
	toolCalls, err := st.Queries().ListMessageToolCalls(ctx, messages[1].ID)
	if err != nil {
		t.Fatalf("ListMessageToolCalls() error = %v", err)
	}
	if len(toolCalls) != 1 || toolCalls[0].ToolName != "Bash" {
		t.Fatalf("toolCalls = %#v", toolCalls)
	}
	if !toolCalls[0].WorkingDirectory.Valid || toolCalls[0].WorkingDirectory.String != "/Users/me/proj" {
		t.Fatalf("tool working dir = %#v", toolCalls[0].WorkingDirectory)
	}
}

func TestParentExternalIDForSubagent(t *testing.T) {
	t.Parallel()

	path := filepath.Join("/tmp", "projects", "-Users-me-proj", "parent-session", "subagents", "agent-a.jsonl")
	parent := parentExternalID(path)
	if !parent.Valid || parent.String != "parent-session" {
		t.Fatalf("parentExternalID() = %#v", parent)
	}
	if !isSubagentPath(path) {
		t.Fatal("expected subagent path")
	}
}

func openTestStore(ctx context.Context, path string) (*store.Store, error) {
	return store.Open(ctx, path)
}
