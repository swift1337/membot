package mcp

import (
	"context"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/swift1337/membot/internal/store"
)

const serverVersion = "dev"

// Run serves membot over MCP on stdin/stdout until the client disconnects.
func Run(ctx context.Context, st *store.Store) error {
	srv := &server{store: st}

	sdkServer := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "membot",
		Version: serverVersion,
	}, nil)

	sdkmcp.AddTool(sdkServer, &sdkmcp.Tool{
		Name:        "search",
		Description: "Search indexed assistant history (messages, memory, artifacts)",
	}, srv.search)

	sdkmcp.AddTool(sdkServer, &sdkmcp.Tool{
		Name:        "list_projects",
		Description: "List indexed projects with conversation counts",
	}, srv.listProjects)

	if err := sdkServer.Run(ctx, &sdkmcp.StdioTransport{}); err != nil {
		return fmt.Errorf("mcp server: %w", err)
	}
	return nil
}

type server struct {
	store *store.Store
}
