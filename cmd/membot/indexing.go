package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/swift1337/membot/internal/indexer"
	"github.com/swift1337/membot/internal/indexer/claude"
	"github.com/swift1337/membot/internal/indexer/codex"
	"github.com/swift1337/membot/internal/indexer/cursor"
	"github.com/swift1337/membot/internal/store"
	"github.com/swift1337/membot/internal/system/macos"
)

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

			result, err := cursor.Index(cmd.Context(), db, cursor.Options{Root: cursorRoot})
			if err != nil {
				return err
			}

			return writeJSON(cmd, result)
		},
	}
	cursorCmd.Flags().StringVar(&cursorRoot, "root", cursor.DefaultRoot(), "Cursor projects root")
	cursorCmd.Flags().BoolVar(&cursorReindex, "reindex", false, "Delete the database file before indexing")

	var (
		claudeRoot    string
		claudeReindex bool
	)
	claudeCmd := &cobra.Command{
		Use:   "claude",
		Short: "Index local Claude Code project transcripts",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(cmd)
			if err != nil {
				return err
			}

			if claudeReindex {
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

			result, err := claude.Index(cmd.Context(), db, claude.Options{Root: claudeRoot})
			if err != nil {
				return err
			}

			return writeJSON(cmd, result)
		},
	}
	claudeCmd.Flags().StringVar(&claudeRoot, "root", claude.DefaultRoot(), "Claude Code data root")
	claudeCmd.Flags().BoolVar(&claudeReindex, "reindex", false, "Delete the database file before indexing")

	var (
		codexRoot    string
		codexReindex bool
	)
	codexCmd := &cobra.Command{
		Use:   "codex",
		Short: "Index local Codex transcripts",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(cmd)
			if err != nil {
				return err
			}

			if codexReindex {
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

			result, err := codex.Index(cmd.Context(), db, codex.Options{Root: codexRoot})
			if err != nil {
				return err
			}

			return writeJSON(cmd, result)
		},
	}
	codexCmd.Flags().StringVar(&codexRoot, "root", codex.DefaultRoot(), "Codex data root")
	codexCmd.Flags().BoolVar(&codexReindex, "reindex", false, "Delete the database file before indexing")

	var (
		watch    bool
		interval time.Duration
		lockPath string
	)
	allCmd := &cobra.Command{
		Use:   "all",
		Short: "Index all configured sources",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openStore(cmd)
			if err != nil {
				return err
			}
			defer func() {
				_ = db.Close()
			}()

			opts := indexer.AllOptions{
				Codex:  indexer.CodexOptions{Root: codexRoot},
				Claude: indexer.ClaudeOptions{Root: claudeRoot},
				Cursor: indexer.CursorOptions{Root: cursorRoot},
			}
			if !watch {
				result, err := indexer.IndexAll(cmd.Context(), db, opts)
				if err != nil {
					return err
				}
				return writeJSON(cmd, result)
			}

			encoder := json.NewEncoder(cmd.OutOrStdout())
			return indexer.Watch(cmd.Context(), db, indexer.WatchOptions{
				Interval:   interval,
				LockPath:   lockPath,
				AllOptions: opts,
			}, func(run indexer.WatchRun) {
				_ = encoder.Encode(run)
			})
		},
	}
	allCmd.Flags().StringVar(&claudeRoot, "claude-root", claude.DefaultRoot(), "Claude Code data root")
	allCmd.Flags().StringVar(&codexRoot, "codex-root", codex.DefaultRoot(), "Codex data root")
	allCmd.Flags().StringVar(&cursorRoot, "cursor-root", cursor.DefaultRoot(), "Cursor projects root")
	allCmd.Flags().BoolVar(&watch, "watch", false, "Keep indexing on an interval until interrupted")
	allCmd.Flags().DurationVar(&interval, "interval", 120*time.Second, "Time between index runs when --watch is set")
	allCmd.Flags().StringVar(&lockPath, "lock", indexer.DefaultLockPath(), "Index lock file path when --watch is set")

	cmd.AddCommand(cursorCmd, claudeCmd, codexCmd, allCmd, newIndexStatsCommand(openStore))
	return cmd
}

func newIndexStatsCommand(openStore func(*cobra.Command) (*store.Store, error)) *cobra.Command {
	return &cobra.Command{
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
			sourceRows, err := db.Queries().ListIndexStatsBySource(cmd.Context())
			if err != nil {
				return err
			}

			dbSize := dbSizeBytes(db.Path())
			lastChangeAt := sqliteText(stats.LastIndexedAt)
			sources := make(map[string]map[string]any, len(sourceRows))
			for _, row := range sourceRows {
				lastIndexedAt := sqliteText(row.LastIndexedAt)
				sources[row.SourceKind] = map[string]any{
					"project_count":      row.ProjectCount,
					"conversation_count": row.ConversationCount,
					"message_count":      row.MessageCount,
					"source_file_count":  row.SourceFileCount,
					"last_change_at":     lastIndexedAt,
					"last_change_ago":    relativeAge(lastIndexedAt),
				}
			}
			return writeJSON(cmd, map[string]any{
				"db_path":            db.Path(),
				"db_size_bytes":      dbSize,
				"db_size":            humanBytes(dbSize),
				"project_count":      stats.ProjectCount,
				"conversation_count": stats.ConversationCount,
				"message_count":      stats.MessageCount,
				"source_file_count":  stats.SourceFileCount,
				"last_change_at":     lastChangeAt,
				"last_change_ago":    relativeAge(lastChangeAt),
				"sources":            sources,
			})
		},
	}
}

func relativeAge(value string) string {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}

	duration := time.Since(parsed)
	if duration < 0 {
		duration = -duration
	}

	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		return pluralizeDuration(int(duration/time.Minute), "minute") + " ago"
	case duration < 24*time.Hour:
		return pluralizeDuration(int(duration/time.Hour), "hour") + " ago"
	case duration < 7*24*time.Hour:
		return pluralizeDuration(int(duration/(24*time.Hour)), "day") + " ago"
	case duration < 30*24*time.Hour:
		return pluralizeDuration(int(duration/(7*24*time.Hour)), "week") + " ago"
	case duration < 365*24*time.Hour:
		return pluralizeDuration(int(duration/(30*24*time.Hour)), "month") + " ago"
	default:
		return pluralizeDuration(int(duration/(365*24*time.Hour)), "year") + " ago"
	}
}

func pluralizeDuration(value int, unit string) string {
	if value == 1 {
		return fmt.Sprintf("1 %s", unit)
	}
	return fmt.Sprintf("%d %ss", value, unit)
}

func humanBytes(bytes int64) string {
	if bytes < 0 {
		return ""
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	value := float64(bytes)
	units := []string{"KB", "MB", "GB", "TB"}
	for _, suffix := range units {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PB", value/unit)
}

func newServiceCommand(dbPath *string) *cobra.Command {
	var (
		membotPath string
		interval   time.Duration
	)

	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage the macOS background indexing service",
	}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install and load the macOS LaunchAgent",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := macos.Install(macos.InstallOptions{
				MembotPath: membotPath,
				DBPath:     *dbPath,
				Interval:   interval,
			})
			if err != nil {
				return err
			}
			return writeJSON(cmd, status)
		},
	}
	installCmd.Flags().StringVar(&membotPath, "membot", "", "Path to the membot binary (default: lookup on PATH)")
	installCmd.Flags().DurationVar(&interval, "interval", 120*time.Second, "Background index interval")

	uninstallCmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Unload and remove the macOS LaunchAgent",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := macos.Uninstall()
			if err != nil {
				return err
			}
			return writeJSON(cmd, status)
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show macOS LaunchAgent status",
		RunE: func(cmd *cobra.Command, args []string) error {
			plistPath, err := macos.DefaultLaunchAgentPath()
			if err != nil {
				return err
			}
			status, err := macos.StatusForPlist(plistPath)
			if err != nil {
				return err
			}
			return writeJSON(cmd, status)
		},
	}

	cmd.AddCommand(installCmd, uninstallCmd, statusCmd)
	return cmd
}
