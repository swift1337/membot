package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	cursorindexer "github.com/swift1337/membot/internal/indexer/cursor"
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
	cmd.AddCommand(newIndexCommand(openStore))

	return cmd
}

func newIndexCommand(openStore func(*cobra.Command) (*store.Store, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index local assistant history",
	}

	var cursorRoot string
	cursorCmd := &cobra.Command{
		Use:   "cursor",
		Short: "Index local Cursor project transcripts",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			result, err := cursorindexer.Index(cmd.Context(), db, cursorindexer.Options{Root: cursorRoot})
			if err != nil {
				return err
			}

			return writeJSON(cmd, result)
		},
	}
	cursorCmd.Flags().StringVar(&cursorRoot, "root", cursorindexer.DefaultRoot(), "Cursor projects root")

	cmd.AddCommand(cursorCmd)
	cmd.AddCommand(&cobra.Command{
		Use:   "stats",
		Short: "Print index statistics as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			stats, err := db.Queries().GetIndexStats(cmd.Context())
			if err != nil {
				return err
			}

			return writeJSON(cmd, map[string]any{
				"db_path":            db.Path(),
				"db_size_bytes":      dbSizeBytes(db.Path()),
				"project_count":      stats.ProjectCount,
				"conversation_count": stats.ConversationCount,
				"message_count":      stats.MessageCount,
				"source_file_count":  stats.SourceFileCount,
				"last_indexed_at":    sqliteText(stats.LastIndexedAt),
			})
		},
	})

	return cmd
}

func writeJSON(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func dbSizeBytes(path string) int64 {
	var size int64
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Stat(candidate)
		if err == nil {
			size += info.Size()
		}
	}
	return size
}

func sqliteText(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return fmt.Sprint(typed)
	}
}
