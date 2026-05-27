package mcp

import (
	"context"
	"encoding/json"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/swift1337/membot/internal/store"
)

func TestMCPServerTools(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	st, err := store.Open(ctx, t.TempDir()+"/membot.db")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})

	srv := &server{store: st}
	sdkServer := sdkmcp.NewServer(&sdkmcp.Implementation{Name: "membot", Version: "test"}, nil)
	sdkmcp.AddTool(sdkServer, &sdkmcp.Tool{Name: "search"}, srv.search)
	sdkmcp.AddTool(sdkServer, &sdkmcp.Tool{Name: "list_projects"}, srv.listProjects)

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "test"}, nil)
	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	if _, err := sdkServer.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server Connect() error = %v", err)
	}
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect() error = %v", err)
	}
	t.Cleanup(func() {
		_ = session.Close()
	})

	listRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "list_projects"})
	if err != nil {
		t.Fatalf("CallTool(list_projects) error = %v", err)
	}
	if listRes.IsError {
		t.Fatalf("list_projects failed: %+v", listRes.Content)
	}

	var projectsOut listProjectsOutput
	if err := decodeStructuredOutput(listRes, &projectsOut); err != nil {
		t.Fatalf("decode list_projects output: %v", err)
	}
	if len(projectsOut.Projects) != 0 {
		t.Fatalf("projects = %#v, want empty slice", projectsOut.Projects)
	}

	searchRes, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name: "search",
		Arguments: map[string]any{
			"query": "sqlite migration",
			"limit": 5,
		},
	})
	if err != nil {
		t.Fatalf("CallTool(search) error = %v", err)
	}
	if searchRes.IsError {
		t.Fatalf("search failed: %+v", searchRes.Content)
	}

	var searchOut struct {
		Result []any `json:"result"`
	}
	if err := decodeStructuredOutput(searchRes, &searchOut); err != nil {
		t.Fatalf("decode search output: %v", err)
	}
	if searchOut.Result == nil {
		t.Fatal("search result is nil, want empty slice")
	}
}

func decodeStructuredOutput(res *sdkmcp.CallToolResult, out any) error {
	if res.StructuredContent == nil {
		return nil
	}
	data, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}
