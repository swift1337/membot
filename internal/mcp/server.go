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
	}, &sdkmcp.ServerOptions{
		Instructions: "membot is the user's long-term memory of previous AI-assistant " +
			"conversations across all their projects. It recalls " +
			"past decisions, prior solutions, and the reasoning behind changes — context that " +
			"exists nowhere in the code or git history. PRIORITIZE searching membot over git " +
			"archaeology (git log, git blame, git grep): when investigating why a file or " +
			"approach exists, search membot FIRST — a past conversation usually explains the " +
			"intent, not just the diff. Also search it before re-deriving prior work, and " +
			"whenever the user references past sessions ('we discussed', 'last time', 'what " +
			"did we decide'). Use search for topics/keywords, search_file_context for a known " +
			"filename or path, list_projects to discover valid project filters.",
	})

	sdkmcp.AddTool(sdkServer, &sdkmcp.Tool{
		Name: "search",
		Description: "Full-text search over long-term memory of previous assistant " +
			"conversations — past decisions, prior solutions, and the reasoning behind " +
			"changes. Prefer this over git log/blame when investigating why something " +
			"exists or was done. Use when you have keywords or topics, not when you " +
			"already know a specific file path or filename. Query supports AND, OR, and " +
			"parentheses, for example: (sqlite OR sqlc) AND migration.",
	}, srv.search)

	sdkmcp.AddTool(sdkServer, &sdkmcp.Tool{
		Name: "list_projects",
		Description: "List indexed projects with conversation counts. Use the returned " +
			"name, slug, or path as the project filter in search and search_file_context.",
	}, srv.listProjects)

	sdkmcp.AddTool(sdkServer, &sdkmcp.Tool{
		Name: "search_file_context",
		Description: "Find previous assistant conversations that referenced, read, edited, " +
			"searched, or patched a file by filename or path. Use when you know a file such " +
			"as cmd_localnet.sh and need prior context about it — prefer this over git " +
			"log/blame on the file, since conversations capture the intent behind changes.",
	}, srv.searchFileContext)

	if err := sdkServer.Run(ctx, &sdkmcp.StdioTransport{}); err != nil {
		return fmt.Errorf("mcp server: %w", err)
	}
	return nil
}

type server struct {
	store *store.Store
}
