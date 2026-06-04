package mcp

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	generateddb "github.com/swift1337/membot/internal/db"
	"github.com/swift1337/membot/internal/query"
)

type searchInput struct {
	Query   string `json:"query" jsonschema:"required search text; supports uppercase AND, OR, and parentheses, e.g. (sqlite OR sqlc) AND migration"`
	Project string `json:"project,omitempty" jsonschema:"optional project name slug or filesystem path to narrow results"`
	Agent   string `json:"agent,omitempty" jsonschema:"optional indexed agent source: cursor, claude, or codex"`
	Since   string `json:"since,omitempty" jsonschema:"optional time window such as yesterday 3h or 1 week"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum hits to return default 20 max 100"`
}

func (s *server) search(ctx context.Context, _ *sdkmcp.CallToolRequest, in searchInput) (*sdkmcp.CallToolResult, query.Response, error) {
	resp, err := query.Search(ctx, s.store, query.Options{
		Query:   in.Query,
		Project: in.Project,
		Agent:   in.Agent,
		Since:   in.Since,
		Limit:   in.Limit,
	})
	if err != nil {
		return nil, query.Response{}, fmt.Errorf("search: %w", err)
	}
	return nil, resp, nil
}

type searchFileContextInput struct {
	FilenameOrPath string `json:"filename_or_path" jsonschema:"required filename basename or path fragment such as cmd_localnet.sh or internal/mcp"`
	Project        string `json:"project,omitempty" jsonschema:"optional but recommended project name slug or path when filenames are common"`
	Agent          string `json:"agent,omitempty" jsonschema:"optional indexed agent source: cursor, claude, or codex"`
	Limit          int    `json:"limit,omitempty" jsonschema:"maximum conversation hits to return default 20 max 100"`
}

func (s *server) searchFileContext(ctx context.Context, _ *sdkmcp.CallToolRequest, in searchFileContextInput) (*sdkmcp.CallToolResult, query.FileContextResponse, error) {
	resp, err := query.SearchFileContext(ctx, s.store, query.FileContextOptions{
		FilenameOrPath: in.FilenameOrPath,
		Project:        in.Project,
		Agent:          in.Agent,
		Limit:          in.Limit,
	})
	if err != nil {
		return nil, query.FileContextResponse{}, fmt.Errorf("search file context: %w", err)
	}
	return nil, resp, nil
}

type listProjectsOutput struct {
	Projects []generateddb.ListProjectSummariesRow `json:"projects" jsonschema:"indexed workspaces with slug path and conversation counts"`
}

func (s *server) listProjects(ctx context.Context, _ *sdkmcp.CallToolRequest, _ struct{}) (*sdkmcp.CallToolResult, listProjectsOutput, error) {
	projects, err := s.store.Queries().ListProjectSummaries(ctx)
	if err != nil {
		return nil, listProjectsOutput{}, fmt.Errorf("list projects: %w", err)
	}
	if projects == nil {
		projects = []generateddb.ListProjectSummariesRow{}
	}
	return nil, listProjectsOutput{Projects: projects}, nil
}
