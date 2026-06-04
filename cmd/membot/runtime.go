package main

import (
	"time"

	"github.com/swift1337/membot/internal/indexer"
	"github.com/swift1337/membot/internal/indexer/claude"
	"github.com/swift1337/membot/internal/indexer/codex"
	"github.com/swift1337/membot/internal/indexer/cursor"
	"github.com/swift1337/membot/internal/store"
)

type config struct {
	DBPath   string
	Query    queryConfig
	Indexing indexingConfig
	Service  serviceConfig
}

type queryConfig struct {
	Project string
	Agent   string
	Since   string
	Text    bool
	Limit   int
	OrderBy string
}

type indexingConfig struct {
	CursorRoot string
	ClaudeRoot string
	CodexRoot  string
	Reindex    bool
	Watch      bool
	Interval   time.Duration
	LockPath   string
}

type serviceConfig struct {
	MembotPath string
	Interval   time.Duration
}

var runtimeConfig = config{
	DBPath: store.DefaultPath(),
	Query: queryConfig{
		Limit:   20,
		OrderBy: "score",
	},
	Indexing: indexingConfig{
		CursorRoot: cursor.DefaultRoot(),
		ClaudeRoot: claude.DefaultRoot(),
		CodexRoot:  codex.DefaultRoot(),
		Interval:   defaultIndexInterval,
		LockPath:   indexer.DefaultLockPath(),
	},
	Service: serviceConfig{
		Interval: defaultServiceInterval,
	},
}
