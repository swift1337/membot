package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/swift1337/membot/internal/indexer/claude"
	"github.com/swift1337/membot/internal/indexer/codex"
	"github.com/swift1337/membot/internal/indexer/cursor"
	"github.com/swift1337/membot/internal/store"
	"golang.org/x/sync/errgroup"
)

type CodexOptions struct {
	Root string
}

type ClaudeOptions struct {
	Root string
}

type CursorOptions struct {
	Root string
}

type AllOptions struct {
	Codex  CodexOptions
	Claude ClaudeOptions
	Cursor CursorOptions
}

type NamedResult struct {
	Name   string `json:"name"`
	Result any    `json:"result"`
}

type AllResult struct {
	Indexers []NamedResult `json:"indexers"`
}

// FoundSourceNames reports which configured local assistant stores exist.
func FoundSourceNames(opts AllOptions) []string {
	found := sourceAvailability(opts)
	names := make([]string, 0, len(found))
	for _, source := range found {
		if source.Found {
			names = append(names, source.DisplayName)
		}
	}
	return names
}

func IndexAll(ctx context.Context, st *store.Store, opts AllOptions) (AllResult, error) {
	g, ctx := errgroup.WithContext(ctx)
	found := sourceAvailability(opts)

	var cursorResult cursor.Result
	if found[0].Found {
		g.Go(func() error {
			result, err := cursor.Index(ctx, st, cursor.Options{Root: opts.Cursor.Root})
			if err != nil {
				return fmt.Errorf("index cursor: %w", err)
			}
			cursorResult = result
			return nil
		})
	}

	var claudeResult claude.Result
	if found[1].Found {
		g.Go(func() error {
			result, err := claude.Index(ctx, st, claude.Options{Root: opts.Claude.Root})
			if err != nil {
				return fmt.Errorf("index claude: %w", err)
			}
			claudeResult = result
			return nil
		})
	}

	var codexResult codex.Result
	if found[2].Found {
		g.Go(func() error {
			result, err := codex.Index(ctx, st, codex.Options{Root: opts.Codex.Root})
			if err != nil {
				return fmt.Errorf("index codex: %w", err)
			}
			codexResult = result
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return AllResult{}, err
	}

	results := make([]NamedResult, 0, 3)
	if found[0].Found {
		results = append(results, NamedResult{Name: "cursor", Result: cursorResult})
	}
	if found[1].Found {
		results = append(results, NamedResult{Name: "claude", Result: claudeResult})
	}
	if found[2].Found {
		results = append(results, NamedResult{Name: "codex", Result: codexResult})
	}
	return AllResult{Indexers: results}, nil
}

type sourceStatus struct {
	DisplayName string
	Found       bool
}

func sourceAvailability(opts AllOptions) []sourceStatus {
	return []sourceStatus{
		{DisplayName: "Cursor", Found: dirExists(defaultString(opts.Cursor.Root, cursor.DefaultRoot()))},
		{DisplayName: "Claude", Found: dirExists(filepath.Join(defaultString(opts.Claude.Root, claude.DefaultRoot()), "projects"))},
		{DisplayName: "Codex", Found: dirExists(filepath.Join(defaultString(opts.Codex.Root, codex.DefaultRoot()), "sessions"))},
	}
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func dirExists(path string) bool {
	path, err := expandPath(path)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func expandPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return filepath.Abs(path)
}
