package cursor

import (
	"context"

	"github.com/swift1337/membot/internal/store"
)

func openTestStore(ctx context.Context, path string) (*store.Store, error) {
	return store.Open(ctx, path)
}
