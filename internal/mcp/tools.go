package mcp

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	generateddb "github.com/swift1337/membot/internal/db"
	"github.com/swift1337/membot/internal/query"
)

type searchInput struct {
	Query   string `json:"query" jsonschema:"search query string"`
	Project string `json:"project,omitempty" jsonschema:"filter by project name slug or path"`
	Since   string `json:"since,omitempty" jsonschema:"only include results since this time (e.g. yesterday 3h 1 week)"`
	Limit   int    `json:"limit,omitempty" jsonschema:"maximum number of results (default 20 max 100)"`
}

func (s *server) search(ctx context.Context, _ *sdkmcp.CallToolRequest, in searchInput) (*sdkmcp.CallToolResult, query.Response, error) {
	resp, err := query.Search(ctx, s.store, query.Options{
		Query:   in.Query,
		Project: in.Project,
		Since:   in.Since,
		Limit:   in.Limit,
	})
	if err != nil {
		return nil, query.Response{}, fmt.Errorf("search: %w", err)
	}
	return nil, resp, nil
}

type listProjectsOutput struct {
	Projects []generateddb.ListProjectSummariesRow `json:"projects" jsonschema:"indexed projects"`
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
