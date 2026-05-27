package query

import (
	"context"
	"database/sql"
	"testing"

	generateddb "github.com/swift1337/membot/internal/db"
	"github.com/swift1337/membot/internal/store"
)

func TestSearchFileContext(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, t.TempDir()+"/membot.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})

	q := st.Queries()
	now := "2026-05-26T20:57:00Z"

	project, err := q.UpsertProject(ctx, generateddb.UpsertProjectParams{
		Slug:        "Users-me-sandbox",
		Name:        sql.NullString{String: "sandbox", Valid: true},
		CanonicalPath: sql.NullString{
			String: "/Users/me/sandbox",
			Valid:  true,
		},
		FirstSeenAt: sql.NullString{String: now, Valid: true},
		LastSeenAt:  sql.NullString{String: now, Valid: true},
	})
	if err != nil {
		t.Fatalf("UpsertProject() error = %v", err)
	}

	source, err := q.UpsertSource(ctx, generateddb.UpsertSourceParams{
		Kind:     "cursor",
		RootPath: "/tmp/cursor",
	})
	if err != nil {
		t.Fatalf("UpsertSource() error = %v", err)
	}

	conversation, err := q.UpsertConversation(ctx, generateddb.UpsertConversationParams{
		SourceID:   source.ID,
		ProjectID:  sql.NullInt64{Int64: project.ID, Valid: true},
		ExternalID: "conv-1",
		Title:      sql.NullString{String: "Localnet script work", Valid: true},
		StartedAt:  sql.NullString{String: now, Valid: true},
		RawPath:    "/tmp/cursor/agent-transcripts/conv-1/conv-1.jsonl",
	})
	if err != nil {
		t.Fatalf("UpsertConversation() error = %v", err)
	}

	message, err := q.UpsertMessage(ctx, generateddb.UpsertMessageParams{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Seq:            1,
		CreatedAt:      sql.NullString{String: now, Valid: true},
		Text:           sql.NullString{String: "Updated cmd_localnet.sh", Valid: true},
		RawJson:        `{}`,
		ContentHash:    "hash-1",
	})
	if err != nil {
		t.Fatalf("UpsertMessage() error = %v", err)
	}

	file, err := q.UpsertFile(ctx, generateddb.UpsertFileParams{
		ProjectID:      sql.NullInt64{Int64: project.ID, Valid: true},
		Path:           "/Users/me/sandbox/ibc/src/cmd_localnet.sh",
		NormalizedPath: sql.NullString{String: "/Users/me/sandbox/ibc/src/cmd_localnet.sh", Valid: true},
		Basename:       sql.NullString{String: "cmd_localnet.sh", Valid: true},
		Kind:           sql.NullString{String: "code_ref", Valid: true},
	})
	if err != nil {
		t.Fatalf("UpsertFile() error = %v", err)
	}

	if _, err := q.CreateFileMention(ctx, generateddb.CreateFileMentionParams{
		FileID:      file.ID,
		MessageID:   sql.NullInt64{Int64: message.ID, Valid: true},
		MentionKind: "code_ref",
		Snippet:     sql.NullString{String: "ensure_attestor_keystore", Valid: true},
	}); err != nil {
		t.Fatalf("CreateFileMention() error = %v", err)
	}

	resp, err := SearchFileContext(ctx, st, FileContextOptions{
		FilenameOrPath: "cmd_localnet.sh",
		Project:        "sandbox",
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("SearchFileContext() error = %v", err)
	}
	if len(resp.Result) != 1 {
		t.Fatalf("len(result) = %d, want 1", len(resp.Result))
	}

	item := resp.Result[0]
	if item.Type != fileContextResultType {
		t.Fatalf("type = %q, want %q", item.Type, fileContextResultType)
	}
	if item.ConversationID != conversation.ID {
		t.Fatalf("conversation_id = %d, want %d", item.ConversationID, conversation.ID)
	}
	if item.Path != "/Users/me/sandbox/ibc/src/cmd_localnet.sh" {
		t.Fatalf("path = %q", item.Path)
	}
	if item.Mentions != 1 {
		t.Fatalf("mentions = %d, want 1", item.Mentions)
	}
	if len(item.MentionKinds) != 1 || item.MentionKinds[0] != "code_ref" {
		t.Fatalf("mention_kinds = %#v", item.MentionKinds)
	}
}

func TestEscapeLikePattern(t *testing.T) {
	t.Parallel()

	if got := escapeLikePattern("100%_done"); got != "100done" {
		t.Fatalf("escapeLikePattern() = %q, want 100done", got)
	}
}

func TestParseMentionKinds(t *testing.T) {
	t.Parallel()

	got := parseMentionKinds("code_ref,tool_read,code_ref")
	want := []string{"code_ref", "tool_read"}
	if len(got) != len(want) {
		t.Fatalf("parseMentionKinds() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kind %d = %q, want %q", i, got[i], want[i])
		}
	}
}
