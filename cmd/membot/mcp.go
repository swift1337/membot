package main

import (
	"github.com/spf13/cobra"

	membotmcp "github.com/swift1337/membot/internal/mcp"
	"github.com/swift1337/membot/internal/store"
)

func newMCPCommand(openStore func(*cobra.Command) (*store.Store, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run membot as an MCP server over stdin/stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			return membotmcp.Run(cmd.Context(), db)
		},
	}
}
