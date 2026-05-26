package query

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	generateddb "github.com/swift1337/membot/internal/db"
	"github.com/swift1337/membot/internal/store"
)

const (
	defaultLimit = 20
	maxLimit     = 100

	relatedFilesLimit = 15
	toolCallsLimit    = 20
)

type hit struct {
	resultType        string
	score             float64
	projectName       string
	projectDir        string
	conversationID    int64
	conversationTitle string
	entityID          int64
	role              string
	createdAt         string
	snippet           string
}

// Search runs a full-text query with optional filters.
func Search(ctx context.Context, st *store.Store, opts Options) (Response, error) {
	query := strings.TrimSpace(opts.Query)
	if query == "" {
		return Response{}, fmt.Errorf("query string is required")
	}

	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return Response{}, fmt.Errorf("query string has no searchable terms")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	since, err := parseSince(opts.Since, time.Now())
	if err != nil {
		return Response{}, err
	}

	q := st.Queries()
	enableProject := int64(0)
	projectID := sql.NullInt64{}
	if strings.TrimSpace(opts.Project) != "" {
		project, err := resolveProject(ctx, q, opts.Project)
		if err != nil {
			return Response{}, err
		}
		enableProject = 1
		projectID = sql.NullInt64{Int64: project.ID, Valid: true}
	}

	enableSince := int64(0)
	sinceParam := sql.NullString{}
	if since != "" {
		enableSince = 1
		sinceParam = sql.NullString{String: since, Valid: true}
	}

	perSourceLimit := int64(limit * 2)
	searchParams := struct {
		fts           string
		enableProject int64
		projectID     sql.NullInt64
		enableSince   int64
		since         sql.NullString
		limit         int64
	}{
		fts:           ftsQuery,
		enableProject: enableProject,
		projectID:     projectID,
		enableSince:   enableSince,
		since:         sinceParam,
		limit:         perSourceLimit,
	}

	messageHits, err := q.SearchMessageHits(ctx, generateddb.SearchMessageHitsParams{
		FtsQuery:      searchParams.fts,
		EnableProject: searchParams.enableProject,
		ProjectID:     searchParams.projectID,
		EnableSince:   searchParams.enableSince,
		Since:         searchParams.since,
		ResultLimit:   searchParams.limit,
	})
	if err != nil {
		return Response{}, fmt.Errorf("search messages: %w", err)
	}

	memoryHits, err := q.SearchMemoryHits(ctx, generateddb.SearchMemoryHitsParams{
		FtsQuery:      searchParams.fts,
		EnableProject: searchParams.enableProject,
		ProjectID:     searchParams.projectID,
		EnableSince:   searchParams.enableSince,
		Since:         searchParams.since,
		ResultLimit:   searchParams.limit,
	})
	if err != nil {
		return Response{}, fmt.Errorf("search memory: %w", err)
	}

	artifactHits, err := q.SearchArtifactHits(ctx, generateddb.SearchArtifactHitsParams{
		FtsQuery:      searchParams.fts,
		EnableProject: searchParams.enableProject,
		ProjectID:     searchParams.projectID,
		EnableSince:   searchParams.enableSince,
		Since:         searchParams.since,
		ResultLimit:   searchParams.limit,
	})
	if err != nil {
		return Response{}, fmt.Errorf("search artifacts: %w", err)
	}

	hits := make([]hit, 0, len(messageHits)+len(memoryHits)+len(artifactHits))
	hits = append(hits, hitsFromMessages(messageHits)...)
	hits = append(hits, hitsFromMemory(memoryHits)...)
	hits = append(hits, hitsFromArtifacts(artifactHits)...)

	sort.Slice(hits, func(i, j int) bool {
		if hits[i].score == hits[j].score {
			return hits[i].createdAt > hits[j].createdAt
		}
		return hits[i].score < hits[j].score
	})

	if len(hits) > limit {
		hits = hits[:limit]
	}

	result := make([]ResultItem, 0, len(hits))
	messageIDs := make([]int64, 0, len(hits))
	for _, item := range hits {
		result = append(result, toResultItem(item))
		if item.resultType == "message" {
			messageIDs = append(messageIDs, item.entityID)
		}
	}

	response := Response{Result: result}
	if len(messageIDs) == 0 {
		return response, nil
	}

	related, err := q.ListRelatedFilesForMessages(ctx, generateddb.ListRelatedFilesForMessagesParams{
		MessageIds: toNullInt64Slice(messageIDs),
		Limit:      relatedFilesLimit,
	})
	if err != nil {
		return Response{}, fmt.Errorf("list related files: %w", err)
	}
	if len(related) > 0 {
		response.RelatedFiles = make([]RelatedFile, 0, len(related))
		for _, row := range related {
			response.RelatedFiles = append(response.RelatedFiles, RelatedFile{
				Path:        row.Path,
				ProjectName: row.ProjectName,
				Mentions:    row.Mentions,
			})
		}
	}

	toolCalls, err := q.ListToolCallsForMessages(ctx, generateddb.ListToolCallsForMessagesParams{
		MessageIds: messageIDs,
		Limit:      toolCallsLimit,
	})
	if err != nil {
		return Response{}, fmt.Errorf("list tool calls: %w", err)
	}
	if len(toolCalls) > 0 {
		response.ToolCalls = make([]ToolCall, 0, len(toolCalls))
		for _, row := range toolCalls {
			response.ToolCalls = append(response.ToolCalls, ToolCall{
				ToolName:  row.ToolName,
				Status:    row.Status,
				CreatedAt: row.CreatedAt,
				MessageID: row.MessageID,
			})
		}
	}

	return response, nil
}

func hitsFromMessages(rows []generateddb.SearchMessageHitsRow) []hit {
	out := make([]hit, 0, len(rows))
	for _, row := range rows {
		out = append(out, hit{
			resultType:        row.ResultType,
			score:             normalizeScore(row.Score),
			projectName:       row.ProjectName,
			projectDir:        row.ProjectDir,
			conversationID:    row.ConversationID,
			conversationTitle: row.ConversationTitle,
			entityID:          row.EntityID,
			role:              row.Role,
			createdAt:         row.CreatedAt,
			snippet:           snippetText(row.Snippet),
		})
	}
	return out
}

func hitsFromMemory(rows []generateddb.SearchMemoryHitsRow) []hit {
	out := make([]hit, 0, len(rows))
	for _, row := range rows {
		out = append(out, hit{
			resultType:     row.ResultType,
			score:          normalizeScore(row.Score),
			projectName:    row.ProjectName,
			projectDir:     row.ProjectDir,
			conversationID: row.ConversationID,
			entityID:       row.EntityID,
			role:           row.Role,
			createdAt:      row.CreatedAt,
			snippet:        snippetText(row.Snippet),
		})
	}
	return out
}

func hitsFromArtifacts(rows []generateddb.SearchArtifactHitsRow) []hit {
	out := make([]hit, 0, len(rows))
	for _, row := range rows {
		out = append(out, hit{
			resultType:     row.ResultType,
			score:          normalizeScore(row.Score),
			projectName:    row.ProjectName,
			projectDir:     row.ProjectDir,
			conversationID: row.ConversationID,
			entityID:       row.EntityID,
			role:           row.Role,
			createdAt:      row.CreatedAt,
			snippet:        snippetText(row.Snippet),
		})
	}
	return out
}

func toResultItem(item hit) ResultItem {
	result := ResultItem{
		Type:              item.resultType,
		Score:             item.score,
		ProjectName:       item.projectName,
		ProjectDir:        item.projectDir,
		ConversationID:    item.conversationID,
		ConversationTitle: item.conversationTitle,
		Role:              item.role,
		CreatedAt:         item.createdAt,
		Snippet:           item.snippet,
	}
	switch item.resultType {
	case "message":
		result.MessageID = item.entityID
	case "memory":
		result.MemoryID = item.entityID
	case "artifact":
		result.ArtifactID = item.entityID
	}
	return result
}

func normalizeScore(raw float64) float64 {
	if raw <= 0 {
		return 1
	}
	return 1 / (1 + raw)
}

func snippetText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	case []byte:
		return strings.TrimSpace(string(typed))
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func toNullInt64Slice(ids []int64) []sql.NullInt64 {
	out := make([]sql.NullInt64, len(ids))
	for i, id := range ids {
		out[i] = sql.NullInt64{Int64: id, Valid: true}
	}
	return out
}
