<p align="center">
  <img src=".github/static/banner.png" height="200" alt="membot banner">
</p>

# Membot

`membot` indexes local assistant history into a memory store so you
can search past conversations, related files, and tool calls.

## Supported agents

- [x] Cursor: project transcripts, tool calls, and repos referenced by VS-code workspaces
- [x] Claude Code: project transcripts from the Claude data directory
- [x] Codex: CLI and Desktop transcripts from the durable Codex data directory

## Features

- [x] CLI to index, query, and inspect local assistant history from the terminal
- [x] Boolean full-text search (`AND`, `OR`, parentheses) with project, source, and time (`--since`) filters
- [x] JSON or formatted (`--text`) output
- [x] File context search to find conversations that touched a given file by name or path
- [x] MCP server exposing the same search over the Model Context Protocol
- [x] Background indexing via a macOS LaunchAgent (`membot service install/uninstall/status`)


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

Index Claude Code project transcripts from the default Claude data directory:

```sh
membot index claude
```

Index Codex transcripts from the default Codex data directory:

```sh
membot index codex
```

Index every supported local assistant source:

```sh
membot index all
```

The Cursor indexer records both transcript workspaces and repos referenced by
`.code-workspace` files. Workspace folders can appear with `0 chats` when Cursor
has no transcripts directly under that workspace, but related chats are still
counted when tool working directories or file mentions point inside that repo.

Use a custom Cursor projects root:

```sh
membot index cursor --root ~/.cursor/projects
```

Use a custom Claude Code data root:

```sh
membot index claude --root ~/.claude
```

Use a custom Codex data root:

```sh
membot index codex --root ~/.codex
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

Use `AND`, `OR`, and parentheses for boolean searches:

```sh
membot query '(sqlite OR sqlc) AND migration' --text
```

Limit results to a project and recent history:

```sh
membot query "tool calls" --project membot --since 1w --limit 10 --text
```

Filter results to one indexed assistant source:

```sh
membot query "workflow" --agent claude --text
membot query "project context" --agent codex --text
membot query files workflow.go --agent cursor --project sandbox --text
```

Return JSON for scripts:

```sh
membot query "cursor transcripts"
```

Search terms are matched against indexed conversation messages with prefix
full-text search. Adjacent terms are combined with `AND`; use uppercase `OR` for
alternatives and parentheses for grouping. Results may include related files and
tool calls linked to matching messages. Tool call `arguments` are decoded as JSON
objects or arrays when possible, with non-JSON values returned as strings.

Find conversations that touched a specific file:

```sh
membot query files cmd_localnet.sh --project sandbox --text
membot query files internal/mcp/tools.go --project membot
```

File context search matches indexed file paths and basenames from code citations,
inline paths, and tool calls (read, write, patch, glob, grep). Use `--project`
when searching common filenames. A project filter matches conversations stored
under that project as well as conversations that touched files or tool working
directories inside the project's canonical path.

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
| `search` | Full-text search over indexed conversation messages. Args: `query` (required; supports `AND`, `OR`, and parentheses like `(sqlite OR sqlc) AND migration`), optional `project`, `agent` (`cursor`, `claude`, or `codex`), `since`, `limit`. Same as `membot query`. |
| `search_file_context` | Find conversations linked to a file by filename or path. Args: `filename_or_path` (required), optional `project`, `agent` (`cursor`, `claude`, or `codex`), `limit`. Same as `membot query files`. |
| `list_projects` | List indexed workspaces with conversation counts. Use names/slugs/paths as `project` filters. Same as `membot query projects`. |

## Development

```sh
make build
make codegen
make lint
```

`make codegen` regenerates the sqlc database package in `internal/db`.
