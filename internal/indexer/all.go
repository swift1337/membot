package indexer

import (
	"context"
	"fmt"

	"github.com/swift1337/membot/internal/indexer/claude"
	"github.com/swift1337/membot/internal/indexer/cursor"
	"github.com/swift1337/membot/internal/store"
)

type ClaudeOptions struct {
	Root string
}

type CursorOptions struct {
	Root string
}

type AllOptions struct {
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
	cursorResult, err := cursor.Index(ctx, st, cursor.Options{Root: opts.Cursor.Root})
	if err != nil {
		return AllResult{}, fmt.Errorf("index cursor: %w", err)
	}
	claudeResult, err := claude.Index(ctx, st, claude.Options{Root: opts.Claude.Root})
	if err != nil {
		return AllResult{}, fmt.Errorf("index claude: %w", err)
	}

	return AllResult{
		Indexers: []NamedResult{
			{
				Name:   "cursor",
				Result: cursorResult,
			},
			{
				Name:   "claude",
				Result: claudeResult,
			},
		},
	}, nil
}
