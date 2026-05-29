package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

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
	cmd.AddCommand(newServiceCommand(&dbPath))
	cmd.AddCommand(newQueryCommand(openStore))
	cmd.AddCommand(newMCPCommand(openStore))

	return cmd
}

func newQueryCommand(openStore func(*cobra.Command) (*store.Store, error)) *cobra.Command {
	var (
		projectFlag string
		agentFlag   string
		sinceFlag   string
		textFlag    bool
		limitFlag   int
		orderByFlag string
	)

	cmd := &cobra.Command{
		Use:     "query [query string]",
		Aliases: []string{"q"},
		Short:   "Query indexed memory as JSON",
		Long: `Search indexed messages, memory, and artifacts.

Query syntax supports AND, OR, and parentheses. Adjacent terms are AND'd.
Use single quotes in the shell when the query contains parentheses.

Examples:
  membot query 'migration sqlc'
  membot query '(foo OR bar) AND fizz' --project membot --text`,
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
				Agent:   agentFlag,
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
	cmd.Flags().StringVar(&agentFlag, "agent", "", "Filter by indexed agent source: cursor or claude")
	cmd.Flags().StringVar(&sinceFlag, "since", "", "Only include results since this time (e.g. yesterday, 3h, 1 week)")
	cmd.Flags().BoolVar(&textFlag, "text", false, "Render results as formatted text instead of JSON")
	cmd.Flags().IntVar(&limitFlag, "limit", 20, "Maximum number of results")
	cmd.Flags().StringVar(&orderByFlag, "order-by", "score", "Sort results by score, date-desc, or date-asc")

	var (
		filesProjectFlag string
		filesAgentFlag   string
		filesTextFlag    bool
		filesLimitFlag   int
		projectsTextFlag bool
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
				Agent:          filesAgentFlag,
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
	filesCmd.Flags().StringVar(&filesAgentFlag, "agent", "", "Filter by indexed agent source: cursor or claude")
	filesCmd.Flags().BoolVar(&filesTextFlag, "text", false, "Render results as formatted text instead of JSON")
	filesCmd.Flags().IntVar(&filesLimitFlag, "limit", 20, "Maximum number of results")
	cmd.AddCommand(filesCmd)

	projectsCmd := &cobra.Command{
		Use:   "projects",
		Short: "List indexed projects",
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

			if projectsTextFlag {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), query.RenderProjectsText(projects))
				return err
			}
			return writeJSON(cmd, projects)
		},
	}
	projectsCmd.Flags().BoolVar(&projectsTextFlag, "text", false, "Render projects as formatted text instead of JSON")
	cmd.AddCommand(projectsCmd)

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
