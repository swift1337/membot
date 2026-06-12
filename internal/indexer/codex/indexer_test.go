package codex

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/swift1337/membot/internal/db"
	"github.com/swift1337/membot/internal/store"
)

func TestConversationIDFromRolloutPath(t *testing.T) {
	t.Parallel()

	path := filepath.Join("/tmp", ".codex", "sessions", "2026", "06", "04", "rollout-2026-06-04T10-20-30Z-thread-abc.jsonl")
	if got := conversationIDFromPath(path); got != "thread-abc" {
		t.Fatalf("conversationIDFromPath() = %q, want thread-abc", got)
	}
}

func TestParseLineIndexesUsefulMessagesOnly(t *testing.T) {
	t.Parallel()

	user, _, ok, err := parseLine([]byte(`{"type":"event_msg","timestamp":"2026-06-04T10:00:00Z","payload":{"type":"user_message","message":"Find `+"`internal/db/db.go`"+`"}}`), 1)
	if err != nil {
		t.Fatalf("parseLine(user) error = %v", err)
	}
	if !ok || user.Role != "user" || user.Text() != "Find `internal/db/db.go`" {
		t.Fatalf("user line = %#v ok=%v", user, ok)
	}

	dev, _, ok, err := parseLine([]byte(`{"type":"response_item","item":{"type":"message","role":"developer","content":[{"type":"input_text","text":"base instructions"}]}}`), 2)
	if err != nil {
		t.Fatalf("parseLine(developer) error = %v", err)
	}
	if ok || dev.Text() != "" {
		t.Fatalf("developer line should be skipped: %#v", dev)
	}

	meta, lineMeta, ok, err := parseLine([]byte(`{"type":"session_meta","payload":{"cwd":"/Users/example/project","base_instructions":"do not index this"}}`), 3)
	if err != nil {
		t.Fatalf("parseLine(session_meta) error = %v", err)
	}
	if ok || meta.Text() != "" || lineMeta.Cwd != "/Users/example/project" {
		t.Fatalf("session_meta line/meta = %#v %#v ok=%v", meta, lineMeta, ok)
	}
}

func TestIndexCodexMissingSessionsIsOptional(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	root := t.TempDir()
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
	if result != (Result{}) {
		t.Fatalf("Index() result = %#v, want empty result", result)
	}
}

func TestIndexCodexTranscriptWithStateMetadata(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	root := t.TempDir()
	sessionDir := filepath.Join(root, "sessions", "2026", "06", "04")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	createStateDB(t, filepath.Join(root, "state_5.sqlite"), "thread-state", "State Title", "/Users/example/state-project")

	transcriptPath := filepath.Join(sessionDir, "rollout-2026-06-04T10-00-00Z-thread-state.jsonl")
	transcript := `{"type":"session_meta","payload":{"thread_id":"thread-state","cwd":"/Users/example/jsonl-project","base_instructions":"never searchable"}}
{"type":"event_msg","timestamp":"2026-06-04T10:00:00Z","payload":{"type":"user_message","message":"Please inspect codex phrase"}}
{"type":"event_msg","timestamp":"2026-06-04T10:00:01Z","payload":{"type":"agent_message","message":"Mirrored response"}}
{"type":"response_item","timestamp":"2026-06-04T10:00:01Z","item":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Single assistant answer"}]}}
{"type":"response_item","timestamp":"2026-06-04T10:00:02Z","item":{"type":"message","role":"system","content":"runtime noise"}}
{"type":"response_item","timestamp":"2026-06-04T10:00:03Z","item":{"type":"function_call","name":"shell","arguments":{"cmd":"go test ./..."},"working_directory":"/Users/example/state-project"}}
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
	if result.FilesDiscovered != 1 || result.FilesIndexed != 1 || result.MessagesIndexed != 3 || result.ParseErrors != 0 {
		t.Fatalf("Index() result = %#v", result)
	}

	source, err := st.Queries().GetSourceByKindRoot(ctx, db.GetSourceByKindRootParams{Kind: sourceKind, RootPath: root})
	if err != nil {
		t.Fatalf("GetSourceByKindRoot() error = %v", err)
	}
	conversation, err := st.Queries().GetConversationByExternalID(ctx, db.GetConversationByExternalIDParams{
		SourceID:   source.ID,
		ExternalID: "thread-state",
	})
	if err != nil {
		t.Fatalf("GetConversationByExternalID() error = %v", err)
	}
	if !conversation.Title.Valid || conversation.Title.String != "State Title" {
		t.Fatalf("conversation title = %#v", conversation.Title)
	}

	project, err := st.Queries().GetProject(ctx, conversation.ProjectID.Int64)
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if !project.CanonicalPath.Valid || project.CanonicalPath.String != "/Users/example/state-project" {
		t.Fatalf("project path = %#v", project.CanonicalPath)
	}

	messages, err := st.Queries().ListConversationMessages(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("messages len = %d, want 3: %#v", len(messages), messages)
	}
	if messages[0].Text.String != "Please inspect codex phrase" || messages[1].Text.String != "Single assistant answer" {
		t.Fatalf("messages = %#v", messages)
	}
	if messages[2].Text.Valid {
		t.Fatalf("tool-call message should not be searchable text: %#v", messages[2].Text)
	}

	toolCalls, err := st.Queries().ListMessageToolCalls(ctx, messages[2].ID)
	if err != nil {
		t.Fatalf("ListMessageToolCalls() error = %v", err)
	}
	if len(toolCalls) != 1 || toolCalls[0].ToolName != "shell" {
		t.Fatalf("toolCalls = %#v", toolCalls)
	}
	if !toolCalls[0].WorkingDirectory.Valid || toolCalls[0].WorkingDirectory.String != "/Users/example/state-project" {
		t.Fatalf("tool working dir = %#v", toolCalls[0].WorkingDirectory)
	}
}

func TestIndexCodexTranscriptFallsBackToSessionIndexAndJSONLMetadata(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	root := t.TempDir()
	sessionDir := filepath.Join(root, "sessions", "2026", "06", "04")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "session_index.jsonl"), []byte(`{"thread_id":"thread-index","thread_name":"Indexed Title","cwd":"/Users/example/index-project"}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(session_index) error = %v", err)
	}

	transcriptPath := filepath.Join(sessionDir, "rollout-2026-06-04T10-00-00Z-thread-index.jsonl")
	transcript := `{"type":"turn_context","payload":{"cwd":"/Users/example/jsonl-project"}}
{"type":"response_item","timestamp":"2026-06-04T10:00:01Z","item":{"type":"message","role":"user","content":"Desktop-originated prompt"}}
`
	if err := os.WriteFile(transcriptPath, []byte(transcript), 0o644); err != nil {
		t.Fatalf("WriteFile(transcript) error = %v", err)
	}

	st, err := openTestStore(ctx, filepath.Join(t.TempDir(), "membot.db"))
	if err != nil {
		t.Fatalf("openTestStore() error = %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})

	if _, err := Index(ctx, st, Options{Root: root}); err != nil {
		t.Fatalf("Index() error = %v", err)
	}
	source, err := st.Queries().GetSourceByKindRoot(ctx, db.GetSourceByKindRootParams{Kind: sourceKind, RootPath: root})
	if err != nil {
		t.Fatalf("GetSourceByKindRoot() error = %v", err)
	}
	conversation, err := st.Queries().GetConversationByExternalID(ctx, db.GetConversationByExternalIDParams{
		SourceID:   source.ID,
		ExternalID: "thread-index",
	})
	if err != nil {
		t.Fatalf("GetConversationByExternalID() error = %v", err)
	}
	if !conversation.Title.Valid || conversation.Title.String != "Indexed Title" {
		t.Fatalf("conversation title = %#v", conversation.Title)
	}
	project, err := st.Queries().GetProject(ctx, conversation.ProjectID.Int64)
	if err != nil {
		t.Fatalf("GetProject() error = %v", err)
	}
	if project.CanonicalPath.String != "/Users/example/index-project" {
		t.Fatalf("project path = %#v", project.CanonicalPath)
	}
}

func createStateDB(t *testing.T, path, id, title, cwd string) {
	t.Helper()

	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()
	if _, err := conn.Exec(`CREATE TABLE threads (id TEXT PRIMARY KEY, title TEXT, cwd TEXT)`); err != nil {
		t.Fatalf("create threads table: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO threads(id, title, cwd) VALUES (?, ?, ?)`, id, title, cwd); err != nil {
		t.Fatalf("insert thread metadata: %v", err)
	}
}

func openTestStore(ctx context.Context, path string) (*store.Store, error) {
	return store.Open(ctx, path)
}
