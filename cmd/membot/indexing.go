package main

import (
	"encoding/json"
	"time"

	"github.com/spf13/cobra"

	"github.com/swift1337/membot/internal/indexer"
	"github.com/swift1337/membot/internal/indexer/cursor"
	"github.com/swift1337/membot/internal/system/macos"
	"github.com/swift1337/membot/internal/store"
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
	allCmd.Flags().StringVar(&cursorRoot, "cursor-root", cursor.DefaultRoot(), "Cursor projects root")
	allCmd.Flags().BoolVar(&watch, "watch", false, "Keep indexing on an interval until interrupted")
	allCmd.Flags().DurationVar(&interval, "interval", 120*time.Second, "Time between index runs when --watch is set")
	allCmd.Flags().StringVar(&lockPath, "lock", indexer.DefaultLockPath(), "Index lock file path when --watch is set")

	cmd.AddCommand(cursorCmd, allCmd, newIndexStatsCommand(openStore))
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

			return writeJSON(cmd, map[string]any{
				"db_path":            db.Path(),
				"db_size_bytes":      dbSizeBytes(db.Path()),
				"project_count":      stats.ProjectCount,
				"conversation_count": stats.ConversationCount,
				"message_count":      stats.MessageCount,
				"source_file_count":  stats.SourceFileCount,
				"last_change_at":     sqliteText(stats.LastIndexedAt),
			})
		},
	}
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
