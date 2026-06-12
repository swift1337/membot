package query

import (
	"context"
	"database/sql"
	"testing"

	generateddb "github.com/swift1337/membot/internal/db"
	"github.com/swift1337/membot/internal/store"
)

func TestSearchFiltersByAgent(t *testing.T) {
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
	project := createTestProject(t, ctx, q)
	cursorMessage := createTestMessage(t, ctx, q, project.ID, "cursor", "workflow from cursor", "cursor-conv")
	claudeMessage := createTestMessage(t, ctx, q, project.ID, "claude", "workflow from claude", "claude-conv")

	allResp, err := Search(ctx, st, Options{Query: "workflow", Limit: 10})
	if err != nil {
		t.Fatalf("Search() all error = %v", err)
	}
	if len(allResp.Result) != 2 {
		t.Fatalf("all len = %d, want 2: %#v", len(allResp.Result), allResp.Result)
	}

	claudeResp, err := Search(ctx, st, Options{Query: "workflow", Agent: "claude", Limit: 10})
	if err != nil {
		t.Fatalf("Search() claude error = %v", err)
	}
	if len(claudeResp.Result) != 1 || claudeResp.Result[0].MessageID != claudeMessage.ID {
		t.Fatalf("claude result = %#v, want message %d", claudeResp.Result, claudeMessage.ID)
	}

	cursorResp, err := Search(ctx, st, Options{Query: "workflow", Agent: "cursor", Limit: 10})
	if err != nil {
		t.Fatalf("Search() cursor error = %v", err)
	}
	if len(cursorResp.Result) != 1 || cursorResp.Result[0].MessageID != cursorMessage.ID {
		t.Fatalf("cursor result = %#v, want message %d", cursorResp.Result, cursorMessage.ID)
	}
}

func createTestProject(t *testing.T, ctx context.Context, q *generateddb.Queries) generateddb.Project {
	t.Helper()

	project, err := q.UpsertProject(ctx, generateddb.UpsertProjectParams{
		Slug:          "Users-example-sample-app",
		Name:          sql.NullString{String: "sample-app", Valid: true},
		CanonicalPath: sql.NullString{String: "/Users/example/sample-app", Valid: true},
		FirstSeenAt:   sql.NullString{String: "2026-05-26T20:57:00Z", Valid: true},
		LastSeenAt:    sql.NullString{String: "2026-05-26T20:57:00Z", Valid: true},
	})
	if err != nil {
		t.Fatalf("UpsertProject() error = %v", err)
	}
	return project
}

func createTestMessage(
	t *testing.T,
	ctx context.Context,
	q *generateddb.Queries,
	projectID int64,
	agent string,
	text string,
	externalID string,
) generateddb.Message {
	t.Helper()

	source, err := q.UpsertSource(ctx, generateddb.UpsertSourceParams{
		Kind:     agent,
		RootPath: "/tmp/" + agent,
	})
	if err != nil {
		t.Fatalf("UpsertSource(%s) error = %v", agent, err)
	}
	conversation, err := q.UpsertConversation(ctx, generateddb.UpsertConversationParams{
		SourceID:   source.ID,
		ProjectID:  sql.NullInt64{Int64: projectID, Valid: true},
		ExternalID: externalID,
		Title:      sql.NullString{String: externalID, Valid: true},
		StartedAt:  sql.NullString{String: "2026-05-26T20:57:00Z", Valid: true},
		RawPath:    "/tmp/" + agent + "/" + externalID + ".jsonl",
	})
	if err != nil {
		t.Fatalf("UpsertConversation(%s) error = %v", agent, err)
	}
	message, err := q.UpsertMessage(ctx, generateddb.UpsertMessageParams{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Seq:            1,
		CreatedAt:      sql.NullString{String: "2026-05-26T20:57:00Z", Valid: true},
		Text:           sql.NullString{String: text, Valid: true},
		RawJson:        `{}`,
		ContentHash:    agent + "-hash",
	})
	if err != nil {
		t.Fatalf("UpsertMessage(%s) error = %v", agent, err)
	}
	return message
}
