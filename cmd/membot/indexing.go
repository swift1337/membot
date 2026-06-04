package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/swift1337/membot/internal/indexer"
	"github.com/swift1337/membot/internal/indexer/claude"
	"github.com/swift1337/membot/internal/indexer/codex"
	"github.com/swift1337/membot/internal/indexer/cursor"
	"github.com/swift1337/membot/internal/store"
)

const defaultIndexInterval = 120 * time.Second

var cmdIndex = &cobra.Command{
	Use:   "index",
	Short: "Index local assistant history",
}

var cmdIndexCursor = &cobra.Command{
	Use:   "cursor",
	Short: "Index local Cursor project transcripts",
	RunE:  runIndexCursor,
}

var cmdIndexClaude = &cobra.Command{
	Use:   "claude",
	Short: "Index local Claude Code project transcripts",
	RunE:  runIndexClaude,
}

var cmdIndexCodex = &cobra.Command{
	Use:   "codex",
	Short: "Index local Codex transcripts",
	RunE:  runIndexCodex,
}

var cmdIndexAll = &cobra.Command{
	Use:   "all",
	Short: "Index all configured sources",
	RunE:  runIndexAll,
}

var cmdIndexStats = &cobra.Command{
	Use:   "stats",
	Short: "Print index statistics as JSON",
	RunE:  runIndexStats,
}

func runIndexCursor(cmd *cobra.Command, args []string) error {
	db, err := openIndexStore(cmd.Context())
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	result, err := cursor.Index(cmd.Context(), db, cursor.Options{Root: runtimeConfig.Indexing.CursorRoot})
	if err != nil {
		return err
	}

	return writeJSON(cmd, result)
}

func runIndexClaude(cmd *cobra.Command, args []string) error {
	db, err := openIndexStore(cmd.Context())
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	result, err := claude.Index(cmd.Context(), db, claude.Options{Root: runtimeConfig.Indexing.ClaudeRoot})
	if err != nil {
		return err
	}

	return writeJSON(cmd, result)
}

func runIndexCodex(cmd *cobra.Command, args []string) error {
	db, err := openIndexStore(cmd.Context())
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	result, err := codex.Index(cmd.Context(), db, codex.Options{Root: runtimeConfig.Indexing.CodexRoot})
	if err != nil {
		return err
	}

	return writeJSON(cmd, result)
}

func openIndexStore(ctx context.Context) (*store.Store, error) {
	db, err := openStore(ctx)
	if err != nil {
		return nil, err
	}
	if !runtimeConfig.Indexing.Reindex {
		return db, nil
	}

	dbPath := db.Path()
	if err := db.Close(); err != nil {
		return nil, err
	}
	if err := store.RemoveDatabaseFiles(dbPath); err != nil {
		return nil, err
	}
	return openStore(ctx)
}

func runIndexAll(cmd *cobra.Command, args []string) error {
	db, err := openStore(cmd.Context())
	if err != nil {
		return err
	}
	defer func() {
		_ = db.Close()
	}()

	opts := indexer.AllOptions{
		Codex:  indexer.CodexOptions{Root: runtimeConfig.Indexing.CodexRoot},
		Claude: indexer.ClaudeOptions{Root: runtimeConfig.Indexing.ClaudeRoot},
		Cursor: indexer.CursorOptions{Root: runtimeConfig.Indexing.CursorRoot},
	}
	if !runtimeConfig.Indexing.Watch {
		result, err := indexer.IndexAll(cmd.Context(), db, opts)
		if err != nil {
			return err
		}
		return writeJSON(cmd, result)
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	return indexer.Watch(cmd.Context(), db, indexer.WatchOptions{
		Interval:   runtimeConfig.Indexing.Interval,
		LockPath:   runtimeConfig.Indexing.LockPath,
		AllOptions: opts,
	}, func(run indexer.WatchRun) {
		_ = encoder.Encode(run)
	})
}

func runIndexStats(cmd *cobra.Command, args []string) error {
	db, err := openStore(cmd.Context())
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
