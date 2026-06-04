package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/swift1337/membot/internal/indexer"
	"github.com/swift1337/membot/internal/indexer/claude"
	"github.com/swift1337/membot/internal/indexer/codex"
	"github.com/swift1337/membot/internal/indexer/cursor"
	"github.com/swift1337/membot/internal/store"
)

var cmdRoot = &cobra.Command{
	Use:           "membot",
	Short:         "Long-term AI memory",
	SilenceUsage:  true,
	SilenceErrors: true,
}

var cmdInit = &cobra.Command{
	Use:   "init",
	Short: "Create and migrate database",
	RunE:  runInit,
}

func main() {
	if err := setup(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := cmdRoot.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func setup() error {
	// Commands
	cmdRoot.AddCommand(cmdInit)

	cmdRoot.AddCommand(cmdIndex)
	cmdIndex.AddCommand(cmdIndexCursor, cmdIndexClaude, cmdIndexCodex, cmdIndexAll, cmdIndexStats)

	cmdRoot.AddCommand(cmdService)
	cmdService.AddCommand(cmdServiceInstall, cmdServiceUninstall, cmdServiceStatus)

	cmdRoot.AddCommand(cmdQuery)
	cmdQuery.AddCommand(cmdQueryFiles, cmdQueryProjects)

	cmdRoot.AddCommand(cmdMCP)

	// Flags
	cmdRoot.PersistentFlags().StringVar(&runtimeConfig.DBPath, "db", store.DefaultPath(), "Database path")

	// Query
	cmdQuery.PersistentFlags().StringVarP(&runtimeConfig.Query.Project, "project", "p", "", "Filter by project name, slug, or path")
	cmdQuery.PersistentFlags().StringVar(&runtimeConfig.Query.Agent, "agent", "", "Filter by indexed agent source: cursor, claude, or codex")
	cmdQuery.PersistentFlags().BoolVar(&runtimeConfig.Query.Text, "text", false, "Render results as formatted text instead of JSON")
	cmdQuery.PersistentFlags().IntVar(&runtimeConfig.Query.Limit, "limit", runtimeConfig.Query.Limit, "Maximum number of results")
	cmdQuery.Flags().StringVar(&runtimeConfig.Query.Since, "since", "", "Only include results since this time (e.g. yesterday, 3h, 1 week)")
	cmdQuery.Flags().StringVar(&runtimeConfig.Query.OrderBy, "order-by", runtimeConfig.Query.OrderBy, "Sort results by score, date-desc, or date-asc")

	cmdIndex.PersistentFlags().BoolVar(&runtimeConfig.Indexing.Reindex, "reindex", false, "Reindex from scratch")
	cmdIndexAll.Flags().StringVar(&runtimeConfig.Indexing.ClaudeRoot, "claude-root", claude.DefaultRoot(), "Claude Code data root")
	cmdIndexAll.Flags().StringVar(&runtimeConfig.Indexing.CodexRoot, "codex-root", codex.DefaultRoot(), "Codex data root")
	cmdIndexAll.Flags().StringVar(&runtimeConfig.Indexing.CursorRoot, "cursor-root", cursor.DefaultRoot(), "Cursor projects root")
	cmdIndexAll.Flags().BoolVar(&runtimeConfig.Indexing.Watch, "watch", false, "Keep indexing on an interval until interrupted")
	cmdIndexAll.Flags().DurationVar(&runtimeConfig.Indexing.Interval, "interval", defaultIndexInterval, "Time between index runs when --watch is set")
	cmdIndexAll.Flags().StringVar(&runtimeConfig.Indexing.LockPath, "lock", indexer.DefaultLockPath(), "Index lock file path when --watch is set")

	cmdServiceInstall.Flags().StringVar(&runtimeConfig.Service.MembotPath, "membot", "", "Path to the membot binary (default: lookup on PATH)")
	cmdServiceInstall.Flags().DurationVar(&runtimeConfig.Service.Interval, "interval", defaultServiceInterval, "Background index interval")

	return nil
}

func runInit(cmd *cobra.Command, args []string) error {
	db, err := openStore(cmd.Context())
	if err != nil {
		return err
	}

	defer func() {
		_ = db.Close()
	}()

	return nil
}

func openStore(ctx context.Context) (*store.Store, error) {
	return store.Open(ctx, runtimeConfig.DBPath)
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
