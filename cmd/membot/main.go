package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	cursorindexer "github.com/swift1337/membot/internal/indexer/cursor"
	"github.com/swift1337/membot/internal/query"
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
			return cmd.Help()
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
	cmd.AddCommand(newQueryCommand(openStore))
	cmd.AddCommand(newMCPCommand(openStore))

	return cmd
}

func newIndexCommand(openStore func(*cobra.Command) (*store.Store, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index local assistant history",
	}

	var (
		cursorRoot    string
		cursorReindex bool
	)
	cursorCmd := &cobra.Command{
		Use:   "cursor",
		Short: "Index local Cursor project transcripts",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(cmd)
			if err != nil {
				return err
			}

			if cursorReindex {
				dbPath := db.Path()
				if err := db.Close(); err != nil {
					return err
				}
				if err := store.RemoveDatabaseFiles(dbPath); err != nil {
					return err
				}
				db, err = openStore(cmd)
				if err != nil {
					return err
				}
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
	cursorCmd.Flags().BoolVar(&cursorReindex, "reindex", false, "Delete the database file before indexing")

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

func newQueryCommand(openStore func(*cobra.Command) (*store.Store, error)) *cobra.Command {
	var (
		projectFlag  string
		sinceFlag    string
		textFlag     bool
		limitFlag    int
		orderByFlag  string
	)

	cmd := &cobra.Command{
		Use:     "query [query string]",
		Aliases: []string{"q"},
		Short:   "Query indexed memory as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			response, err := query.Search(cmd.Context(), db, query.Options{
				Query:   strings.Join(args, " "),
				Project: projectFlag,
				Since:   sinceFlag,
				Limit:   limitFlag,
				OrderBy: query.OrderBy(orderByFlag),
			})
			if err != nil {
				return err
			}

			if textFlag {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), query.RenderText(response))
				return err
			}
			return writeJSON(cmd, response)
		},
	}

	cmd.Flags().StringVarP(&projectFlag, "project", "p", "", "Filter by project name, slug, or path")
	cmd.Flags().StringVar(&sinceFlag, "since", "", "Only include results since this time (e.g. yesterday, 3h, 1 week)")
	cmd.Flags().BoolVar(&textFlag, "text", false, "Render results as formatted text instead of JSON")
	cmd.Flags().IntVar(&limitFlag, "limit", 20, "Maximum number of results")
	cmd.Flags().StringVar(&orderByFlag, "order-by", "score", "Sort results by score, date-desc, or date-asc")

	var (
		filesProjectFlag string
		filesTextFlag    bool
		filesLimitFlag   int
	)
	filesCmd := &cobra.Command{
		Use:     "files <filename-or-path>",
		Aliases: []string{"f"},
		Short:   "Find conversations that referenced a file by name or path",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}

			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			response, err := query.SearchFileContext(cmd.Context(), db, query.FileContextOptions{
				FilenameOrPath: strings.Join(args, " "),
				Project:        filesProjectFlag,
				Limit:          filesLimitFlag,
			})
			if err != nil {
				return err
			}

			if filesTextFlag {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), query.RenderFileContextText(response))
				return err
			}
			return writeJSON(cmd, response)
		},
	}
	filesCmd.Flags().StringVarP(&filesProjectFlag, "project", "p", "", "Filter by project name, slug, or path")
	filesCmd.Flags().BoolVar(&filesTextFlag, "text", false, "Render results as formatted text instead of JSON")
	filesCmd.Flags().IntVar(&filesLimitFlag, "limit", 20, "Maximum number of results")
	cmd.AddCommand(filesCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "projects",
		Short: "List indexed projects as JSON",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			projects, err := db.Queries().ListProjectSummaries(cmd.Context())
			if err != nil {
				return err
			}

			return writeJSON(cmd, projects)
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
