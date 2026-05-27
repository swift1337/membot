# Membot

`membot` indexes local assistant history into a memory store so you
can search past conversations, related files, and tool calls.

## Install

```sh
make install
```

This installs the `membot` CLI with `go install ./cmd/membot`. For a local
binary instead, run:

```sh
make build
./bin/membot --help
```

By default, `membot` creates and migrates its SQLite database at
`~/.membot/membot.db`. Use `--db /path/to/membot.db` to override it.

## Index

Index Cursor project transcripts from the default Cursor projects directory:

```sh
membot index cursor
```

Use a custom Cursor projects root:

```sh
membot index cursor --root ~/.cursor/projects
```

Rebuild the database from scratch:

```sh
membot index cursor --reindex
```

Check what has been indexed:

```sh
membot index stats
membot query projects
```

## Query

Show formatted terminal output:

```sh
membot query "sqlite migration" --text
```

Limit results to a project and recent history:

```sh
membot query "tool calls" --project membot --since 1w --limit 10 --text
```

Return JSON for scripts:

```sh
membot query "cursor transcripts"
```

Search terms are matched with prefix full-text search. Results may include
conversation hits, related files, and tool calls linked to matching messages.

## Development

```sh
make build
make codegen
make lint
```

`make codegen` regenerates the sqlc database package in `internal/db`.
