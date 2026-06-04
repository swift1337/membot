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
		Name: "search",
		Description: "Full-text search over indexed assistant conversation messages. Use when " +
			"you have keywords or topics, not when you already know a specific file path or " +
			"filename. Query supports AND, OR, and parentheses, for example: " +
			"(sqlite OR sqlc) AND migration.",
	}, srv.search)

	sdkmcp.AddTool(sdkServer, &sdkmcp.Tool{
		Name: "list_projects",
		Description: "List indexed projects with conversation counts. Use the returned " +
			"name, slug, or path as the project filter in search and search_file_context.",
	}, srv.listProjects)

	sdkmcp.AddTool(sdkServer, &sdkmcp.Tool{
		Name: "search_file_context",
		Description: "Find conversations that referenced, read, edited, searched, or patched a " +
			"file by filename or path. Use when you know a file such as cmd_localnet.sh and " +
			"need prior assistant context about it.",
	}, srv.searchFileContext)

	if err := sdkServer.Run(ctx, &sdkmcp.StdioTransport{}); err != nil {
		return fmt.Errorf("mcp server: %w", err)
	}
	return nil
}

type server struct {
	store *store.Store
}
