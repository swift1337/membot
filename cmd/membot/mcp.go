package main

import (
	"github.com/spf13/cobra"

	"github.com/swift1337/membot/internal/mcp"
	"github.com/swift1337/membot/internal/store"
)

var cmdMCP = &cobra.Command{
	Use:   "mcp",
	Short: "Run MCP server over stdin/stdout",
	RunE:  runMCP,
}

func runMCP(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	db, err := store.Open(ctx, runtimeConfig.DBPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	return mcp.Run(ctx, db)
}
