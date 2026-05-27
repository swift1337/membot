package query

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	generateddb "github.com/swift1337/membot/internal/db"
	"github.com/swift1337/membot/internal/store"
)

const fileContextResultType = "file_context"

// FileContextOptions configures a filename/path context search.
type FileContextOptions struct {
	FilenameOrPath string
	Project        string
	Limit          int
}

// FileContextResponse is the top-level file context search response.
type FileContextResponse struct {
	Query   string             `json:"query"`
	Project string             `json:"project,omitempty"`
	Result  []FileContextItem  `json:"result"`
}

// FileContextItem is a conversation linked to a matched file path.
type FileContextItem struct {
	Type              string   `json:"type"`
	Path              string   `json:"path"`
	ProjectName       string   `json:"project_name"`
	ProjectDir        string   `json:"project_dir"`
	ConversationID    int64    `json:"conversation_id"`
	ConversationTitle string   `json:"conversation_title"`
	Mentions          int64    `json:"mentions"`
	LastMentionedAt   string   `json:"last_mentioned_at,omitempty"`
	MentionKinds      []string `json:"mention_kinds,omitempty"`
	Snippet           string   `json:"snippet,omitempty"`
}

// SearchFileContext finds conversations that referenced a file by basename or path.
func SearchFileContext(ctx context.Context, st *store.Store, opts FileContextOptions) (FileContextResponse, error) {
	pattern := strings.TrimSpace(opts.FilenameOrPath)
	if pattern == "" {
		return FileContextResponse{}, fmt.Errorf("filename or path is required")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}

	q := st.Queries()
	enableProject := int64(0)
	projectID := sql.NullInt64{}
	resolvedProject := strings.TrimSpace(opts.Project)
	if resolvedProject != "" {
		project, err := resolveProject(ctx, q, resolvedProject)
		if err != nil {
			return FileContextResponse{}, err
		}
		enableProject = 1
		projectID = sql.NullInt64{Int64: project.ID, Valid: true}
	}

	normalized := filepath.ToSlash(filepath.Clean(pattern))
	basename := filepath.Base(normalized)
	likePattern := escapeLikePattern(normalized)

	rows, err := q.SearchFileContext(ctx, generateddb.SearchFileContextParams{
		EnableProject: enableProject,
		ProjectID:     projectID,
		Basename:      basename,
		PathContains:  sql.NullString{String: "%" + likePattern + "%", Valid: true},
		PathSuffix:    sql.NullString{String: "%/" + escapeLikePattern(basename), Valid: basename != ""},
		ResultLimit:   int64(limit),
	})
	if err != nil {
		return FileContextResponse{}, fmt.Errorf("search file context: %w", err)
	}

	result := make([]FileContextItem, 0, len(rows))
	for _, row := range rows {
		path := row.Path
		if row.NormalizedPath != "" {
			path = row.NormalizedPath
		}
		result = append(result, FileContextItem{
			Type:              fileContextResultType,
			Path:              path,
			ProjectName:       row.ProjectName,
			ProjectDir:        row.ProjectDir,
			ConversationID:    row.ConversationID,
			ConversationTitle: row.ConversationTitle,
			Mentions:          row.Mentions,
			LastMentionedAt:   sqliteText(row.LastMentionedAt),
			MentionKinds:      parseMentionKinds(row.MentionKinds),
			Snippet:           snippetText(sqliteText(row.Snippet)),
		})
	}

	return FileContextResponse{
		Query:   pattern,
		Project: resolvedProject,
		Result:  result,
	}, nil
}

func escapeLikePattern(value string) string {
	value = strings.ReplaceAll(value, "%", "")
	value = strings.ReplaceAll(value, "_", "")
	return value
}

func sqliteText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(typed)
	}
}

func parseMentionKinds(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		out = append(out, part)
	}
	return out
}
