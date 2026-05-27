package indexer

import (
	"context"
	"fmt"

	"github.com/swift1337/membot/internal/indexer/cursor"
	"github.com/swift1337/membot/internal/store"
)

type CursorOptions struct {
	Root string
}

type AllOptions struct {
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

	return AllResult{
		Indexers: []NamedResult{{
			Name:   "cursor",
			Result: cursorResult,
		}},
	}, nil
}
