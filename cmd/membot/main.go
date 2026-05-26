package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/swift1337/membot/internal/store"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var dbPath string

	openStore := func(cmd *cobra.Command) (*store.Store, error) {
		return store.Open(cmd.Context(), dbPath)
	}

	cmd := &cobra.Command{
		Use:           "membot",
		Short:         "Index local assistant history into searchable memory",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "membot database ready: %s\n", db.Path())
			return err
		},
	}

	cmd.PersistentFlags().StringVar(&dbPath, "db", store.DefaultPath(), "SQLite database path")

	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create and migrate the membot database",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			_, err = fmt.Fprintf(cmd.OutOrStdout(), "membot database ready: %s\n", db.Path())
			return err
		},
	})

	return cmd
}
