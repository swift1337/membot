package query

// OrderBy selects how search hits are sorted.
type OrderBy string

const (
	OrderByScore    OrderBy = "score"
	OrderByDateDesc OrderBy = "date-desc"
	OrderByDateAsc  OrderBy = "date-asc"
)

// Options configures a search query.
type Options struct {
	Query   string
	Project string
	Since   string
	Limit   int
	OrderBy OrderBy
}

// Response is the top-level search response.
type Response struct {
	Result       []ResultItem  `json:"result"`
	RelatedFiles []RelatedFile `json:"related_files,omitempty"`
	ToolCalls    []ToolCall    `json:"tool_calls,omitempty"`
}

// ResultItem is a single ranked search hit.
type ResultItem struct {
	Type              string  `json:"type"`
	Score             float64 `json:"score"`
	ProjectName       string  `json:"project_name"`
	ProjectDir        string  `json:"project_dir"`
	ConversationID    int64   `json:"conversation_id,omitempty"`
	ConversationTitle string  `json:"conversation_title,omitempty"`
	MessageID         int64   `json:"message_id,omitempty"`
	MemoryID          int64   `json:"memory_id,omitempty"`
	ArtifactID        int64   `json:"artifact_id,omitempty"`
	Role              string  `json:"role,omitempty"`
	CreatedAt         string  `json:"created_at,omitempty"`
	Snippet           string  `json:"snippet"`
}

// RelatedFile is a file referenced by matched messages.
type RelatedFile struct {
	Path        string `json:"path"`
	ProjectName string `json:"project_name"`
	Mentions    int64  `json:"mentions"`
}

// ToolCall is a tool invocation linked to a matched conversation.
type ToolCall struct {
	ToolName          string `json:"tool_name"`
	Arguments         string `json:"arguments,omitempty"`
	WorkingDirectory  string `json:"working_directory,omitempty"`
	Status            string `json:"status,omitempty"`
	CreatedAt         string `json:"created_at,omitempty"`
	MessageID         int64  `json:"message_id"`
}
