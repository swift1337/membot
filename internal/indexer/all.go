package indexer

import (
	"context"
	"fmt"

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

func IndexAll(ctx context.Context, st *store.Store, opts AllOptions) (AllResult, error) {
	g, ctx := errgroup.WithContext(ctx)

	var cursorResult cursor.Result
	g.Go(func() error {
		result, err := cursor.Index(ctx, st, cursor.Options{Root: opts.Cursor.Root})
		if err != nil {
			return fmt.Errorf("index cursor: %w", err)
		}
		cursorResult = result
		return nil
	})

	var claudeResult claude.Result
	g.Go(func() error {
		result, err := claude.Index(ctx, st, claude.Options{Root: opts.Claude.Root})
		if err != nil {
			return fmt.Errorf("index claude: %w", err)
		}
		claudeResult = result
		return nil
	})

	var codexResult codex.Result
	g.Go(func() error {
		result, err := codex.Index(ctx, st, codex.Options{Root: opts.Codex.Root})
		if err != nil {
			return fmt.Errorf("index codex: %w", err)
		}
		codexResult = result
		return nil
	})

	if err := g.Wait(); err != nil {
		return AllResult{}, err
	}

	return AllResult{
		Indexers: []NamedResult{
			{Name: "cursor", Result: cursorResult},
			{Name: "claude", Result: claudeResult},
			{Name: "codex", Result: codexResult},
		},
	}, nil
}
