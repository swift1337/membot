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

## MCP

Run membot as an MCP server so Cursor (or any MCP client) can search indexed
history without shelling out to the CLI:

```sh
membot mcp
```

The server speaks MCP over stdin/stdout and exits when the client disconnects.
Use `--db` to point at a non-default database, same as other commands.

### Cursor setup

After `make install`, add this to your Cursor MCP config
(`~/.cursor/mcp.json` or project `.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "membot": {
      "command": "membot",
      "args": ["mcp"]
    }
  }
}
```

If `membot` is not on your `PATH`, use the full path to the binary instead
(for example, the output of `go env GOPATH` plus `/bin/membot`).

Index transcripts first (`membot index cursor`); the MCP server is read-only
and serves whatever is already in the database.

### Tools

| Tool | Description |
| --- | --- |
| `search` | Full-text search over indexed history. Args: `query` (required), optional `project`, `since`, `limit`. Same behavior as `membot query`. |
| `list_projects` | List indexed projects with conversation counts. Same behavior as `membot query projects`. |

## Development

```sh
make build
make codegen
make lint
```

`make codegen` regenerates the sqlc database package in `internal/db`.
