# Cursor transcript indexing

This document explains how Cursor stores assistant history on disk and how
`membot` discovers, parses, and indexes it. It is written for agents working
on the indexer, query layer, or MCP tools.

## Where Cursor stores data

Cursor keeps per-workspace metadata under the user's home directory:

```
~/.cursor/projects/
├── Users-alice-Code-myapp/          # workspace slug (see below)
│   ├── agent-transcripts/           # indexed by membot
│   ├── agent-tools/                 # not indexed (tool output cache)
│   ├── mcps/                        # not indexed (MCP tool descriptors)
│   ├── terminals/                   # not indexed (terminal session snapshots)
│   └── canvases/                    # not indexed
├── Users-alice-Code-other-repo/
│   └── ...
└── 1775236909581/                   # numeric slug (often no transcripts)
    └── terminals/
```

**Default root:** `~/.cursor/projects` (`cursor.DefaultRoot()`).

The `membot index cursor` command uses this default root. To index a custom
Cursor root, use `membot index all --cursor-root /path/to/projects`.

### Project slugs

Each subdirectory of the projects root is a **project slug**. Cursor encodes
the workspace path in the slug by replacing path separators with hyphens:

| Slug | Inferred canonical path |
|------|-------------------------|
| `Users-alice-Code-membot` | `/Users/alice/Code/membot` |
| `Volumes-External-myproj` | `/Volumes/External/myproj` |

Slugs starting with `Users-` or `Volumes-` are treated as path-encoded
workspaces. The indexer converts them back with
`strings.ReplaceAll(slug, "-", "/")` and prepends `/`.

Some slugs are opaque numeric IDs (for example `1775236909581`). These usually
contain only `terminals/` and no `agent-transcripts/`. They are skipped unless
they match the `Users-` / `Volumes-` prefix heuristic.

A directory is considered a Cursor project when either:

1. It contains an `agent-transcripts/` subdirectory, or
2. Its slug starts with `Users-` or `Volumes-`.

Path-encoded projects are upserted even when they have no transcripts. If the
canonical path contains `.code-workspace` files, the indexer also parses their
`folders` entries and upserts each existing local folder as a project. This
lets multi-repo Cursor workspaces expose repos such as `/Users/alice/Code/api`
and `/Users/alice/Code/web` even when Cursor stores the actual transcript under
one workspace slug. Workspace files are parsed as JSON with a small JSONC
tolerance for comments and trailing commas.

### Transcript file layout

Transcripts live under `agent-transcripts/` as **JSONL** files (one JSON object
per line):

```
agent-transcripts/
├── <uuid>/
│   ├── <uuid>.jsonl                 # parent conversation
│   └── subagents/
│       └── <subagent-uuid>.jsonl    # spawned subagent run
└── <another-uuid>/
    └── <another-uuid>.jsonl
```

Conventions:

- **Conversation ID** = basename of the `.jsonl` file without extension (a UUID).
- **Parent link** = the UUID directory immediately under `agent-transcripts/`
  when the file is in a `subagents/` subdirectory.
- **Subagent flag** = path contains `/subagents/`.

Example:

```
.../agent-transcripts/185b46c7-.../185b46c7-....jsonl          # parent
.../agent-transcripts/185b46c7-.../subagents/fc5c473c-....jsonl # subagent
```

Other Cursor directories under the same project root are **not** read by the
indexer today:

| Directory | Contents |
|-----------|----------|
| `agent-tools/` | Cached text output from tool runs (UUID-named `.txt` files) |
| `mcps/` | MCP server tool JSON descriptors for the workspace |
| `terminals/` | Terminal session metadata and scrollback (`.txt` per session) |
| `canvases/` | Canvas artifacts |

## Transcript line format

Each non-empty line is a JSON object. The parser (`cursor-jsonl-v5`) expects:

```json
{
  "role": "user",
  "timestamp": "...",
  "created_at": "...",
  "createdAt": "...",
  "message": {
    "content": "... or [...]",
    "timestamp": "...",
    "created_at": "...",
    "createdAt": "..."
  }
}
```

Only `role` and `message.content` are required for a successful parse. Missing
`role` defaults to `"unknown"`.

### Message content shapes

`message.content` is either:

1. **A plain string** — stored as a single `text` block.
2. **An array of blocks** — each block has at least `type` and often `text`:

```json
{
  "role": "assistant",
  "message": {
    "content": [
      {"type": "text", "text": "I'll look that up."},
      {"type": "tool_use", "name": "Read", "input": {"path": "..."}}
    ]
  }
}
```

Common block types seen in transcripts:

| `type` | Typical fields | Indexed as |
|--------|----------------|------------|
| `text` | `text` | Searchable message text + `message_blocks` row |
| `tool_use` | `name`, `input` | `message_blocks` row; `tool_calls` row; file paths → `files` + `file_mentions` |

The flattened searchable **message text** is all block `text` fields joined
with `\n\n`. Tool `input` JSON is preserved in `message_blocks.raw_json` and
also parsed into `tool_calls` (see below).

### Timestamps

Cursor timestamps appear in several places. The parser collects candidates from
(in order of scan):

- Top-level: `timestamp`, `created_at`, `createdAt`
- Nested under `message`: same three fields
- `<timestamp>...</timestamp>` tags inside `text` block bodies

Supported formats:

- RFC3339 (for example `2026-05-26T20:57:00Z`)
- Human-readable with UTC offset, for example
  `Wednesday, Apr 29, 2026, 5:43 PM (UTC+2)`

If no timestamp is found on any line, conversation start/end fall back to the
source file's modification time.

### Embedded XML-style tags in user messages

User messages often wrap the actual prompt in tags:

```json
{"type":"text","text":"<user_query>\nhello\n</user_query>"}
```

The indexer stores the human-readable content only: `<user_query>` is unwrapped
to its inner text, and `<timestamp>` tags are removed after their value is used
for timestamp extraction. Raw JSON is still preserved on the message and block
records for provenance.

## How membot indexes transcripts

Entry point: `cursor.Index(ctx, store, Options{Root})`.

CLI: `membot index cursor [--reindex]`.

### Pipeline overview

```
discover(root)
  → list project dirs
  → parse .code-workspace folder entries for multi-repo workspaces
  → upsert discovered projects, even when they have zero transcript files
  → walk agent-transcripts/**/*.jsonl
  → for each file: indexTranscript (skip if unchanged)
       → upsert project, source_file, conversation
       → parse JSONL lines → messages + message_blocks
       → extract file mentions from message text → files + file_mentions
       → index tool_use blocks → tool_calls + file mentions from tool paths
       → record parse_errors for bad lines
```

### Incremental indexing

A transcript is **skipped** when an existing `source_files` row matches:

- Same `source_id` + absolute path
- Same `size_bytes`, `mtime_unix`, `sha256`
- Same `parser_version` (`cursor-jsonl-v5`)

Any change to file content, mtime, or parser version triggers a full re-index
of that file inside a single DB transaction (upserts replace prior rows for
that conversation via sqlc upsert keys).

`--reindex` deletes the entire SQLite database and rebuilds from scratch.

### SQLite mapping

| Cursor concept | DB table | Key fields |
|----------------|----------|------------|
| Projects root | `sources` | `kind=cursor`, `root_path` |
| Workspace | `projects` | `slug`, `canonical_path`, `name` |
| `.jsonl` file | `source_files` | `path`, `sha256`, `parser_version` |
| Chat session | `conversations` | `external_id` (UUID), `parent_external_id`, `is_subagent`, `raw_path` |
| JSONL line | `messages` | `seq` (1-based), `role`, `text`, `raw_json`, `raw_line` |
| Content block | `message_blocks` | `seq`, `type`, `text`, `raw_json` |
| Tool invocation | `tool_calls` | `tool_name`, `arguments_json`, `working_directory`, `status` |
| Path in message text | `files` + `file_mentions` | extracted heuristically; `mention_kind` = `code_ref`, `inline_code`, `at_ref` |
| Path from tool call | `files` + `file_mentions` | linked via `tool_call_id`; `mention_kind` = `tool_read`, `tool_write`, `tool_patch` |
| Indexed file path | `files` | `path`, `normalized_path`, `basename` (indexed for lookup) |

Conversation **title** = first non-empty `user` message text, truncated to 120
characters (newlines replaced with spaces).

FTS: inserting/updating `messages.text` triggers `message_fts` via DB triggers
(defined in `001_initial.sql`).

Project summaries and project filters use two relationships:

- **Transcript ownership**: the conversation's Cursor transcript is stored under
  that project slug.
- **Related work**: a tool working directory or indexed file mention is inside
  the project's canonical path.

This matters for multi-repo workspaces: Cursor may store a chat under an `ibc`
workspace while the tool calls and file mentions point at
`/Users/alice/Code/sandbox`. `membot query projects` counts that chat for the
related `sandbox` project, and `--project sandbox` can find it through the file
or working-directory relationship.

### File mention extraction

After each message is stored, `extractFileMentions(text)` scans flattened text
for three patterns:

| Pattern | Example | `mention_kind` |
|---------|---------|----------------|
| Code citation fence | `` ```247:265:/path/to/file.go `` | `code_ref` (+ line range) |
| Inline backticks | `` `src/main.go` `` | `inline_code` |
| At-path reference | `@sandbox/plan.md` | `at_ref` |

Filters (paths that are dropped):

- No `/` in the candidate (not a file path)
- Contains `://` (URLs)
- Contains whitespace
- Normalizes to `.` or `/` only

Paths are cleaned (trim punctuation, strip leading `@`, remove `123:456:` line
prefixes) and stored with `filepath.ToSlash(filepath.Clean(path))`. Each
`files` row also stores `basename` (the final path component) for fast lookup.

### Tool call file extraction

For each `tool_use` block, `indexToolCall` in `tools.go` creates a
`tool_calls` row and extracts file paths from the tool input:

| Tool(s) | Source field | `mention_kind` |
|---------|--------------|----------------|
| Read, ReadFile, ReadLints | `path` | `tool_read` |
| Write, Delete, StrReplace | `path` | `tool_write` |
| Glob, Grep, rg | `path` or `target_directory` | `tool_read` |
| ApplyPatch | patch headers (`*** Update File: ...`) | `tool_patch` |
| Shell | — | not indexed (no file extraction) |

ApplyPatch arguments are compacted to `{"files":["/path/a.go",...]}` in
`tool_calls.arguments_json`. Shell commands store `working_directory` on the
tool call row but do not create file mentions.

Search responses decode `tool_calls.arguments_json` into JSON objects or arrays
when possible. Invalid or non-JSON argument strings are returned unchanged as
strings, so existing compacted/plain values remain representable.

### File context search

Indexed file paths power a separate lookup path from full-text search:

```sh
membot query files cmd_localnet.sh --project sandbox --text
membot query files internal/mcp/tools.go --project membot
```

MCP equivalent: `search_file_context` with `filename_or_path` and optional
`project`.

The query matches `files.basename`, path suffix, and path substring, then joins
through `file_mentions` → `messages` → `conversations`. Results include
conversation title, mention count, mention kinds, and a snippet.

When `--project` is provided, file context search matches both files whose
stored `project_id` is that project and file paths that live under the
project's canonical path. This keeps workspace-folder repos searchable even when
Cursor stored the transcript under a different project.

Coverage limits (today):

- Bare filenames in message text (no `/`) are dropped by `cleanMentionPath`.
- Files referenced only inside shell commands are not indexed.
- The same logical file may appear under both absolute and relative paths
  depending on how it was cited in the transcript.

### Parse errors

Malformed JSON lines are **not** fatal. They are recorded in `parse_errors`
with line number, error text, and a hash of the raw line. Valid lines in the
same file are still indexed.

### Index result JSON

`membot index cursor` prints:

```json
{
  "projects_discovered": 42,
  "projects_indexed": 10,
  "files_discovered": 300,
  "files_indexed": 5,
  "files_skipped": 295,
  "messages_indexed": 1200,
  "parse_errors": 2
}
```

`projects_indexed` counts distinct discovered project slugs upserted in this
run, including `.code-workspace` folders and projects with no transcript files.
`files_indexed` / `files_skipped` describe transcript files only.

## Code map

| File | Responsibility |
|------|----------------|
| `internal/indexer/cursor/indexer.go` | Discovery, parsing, message file mentions, DB writes |
| `internal/indexer/cursor/tools.go` | Tool call indexing and tool-path file extraction |
| `internal/indexer/cursor/indexer_test.go` | Timestamp, mention, and tool parsing tests |
| `cmd/membot/main.go` | CLI commands (`index cursor`, `query`, `query files`, `mcp`) |
| `internal/store/migrations/001_initial.sql` | Schema, FTS triggers, and `files.basename` |
| `internal/query/search.go` | Full-text search over messages, memory, artifacts |
| `internal/query/file_context.go` | File-to-conversation lookup |
| `internal/mcp/` | MCP server exposing `search`, `search_file_context`, `list_projects` |

Key constants in `indexer.go`:

```go
sourceKind    = "cursor"
parserVersion = "cursor-jsonl-v5"
```

Bump `parserVersion` when parsing logic changes so existing files are re-read.

## Agent workflow notes

1. **Refresh index** after new Cursor chats: `membot index cursor` (incremental).
2. **Full-text query** indexed history: `membot query "..."` or MCP `search`.
3. **File context query** when you know a filename: `membot query files <name>`
   or MCP `search_file_context`. Use `--project` for common filenames.
4. **Subagent transcripts** are indexed as separate conversations with
   `is_subagent=1` and `parent_external_id` pointing at the parent UUID.
5. **Do not assume** `agent-tools/`, `terminals/`, or `mcps/` are searchable;
   only `agent-transcripts/**/*.jsonl` is ingested today.
6. When changing the parser, update tests in `indexer_test.go`, bump
   `parserVersion`, and run `make lint` + `go test ./internal/indexer/cursor/...`.

## Example transcript snippets

**User message with tagged query:**

```json
{"role":"user","message":{"content":[{"type":"text","text":"<user_query>\nAdd MCP support\n</user_query>"}]}}
```

**Assistant with text + tool call:**

```json
{"role":"assistant","message":{"content":[
  {"type":"text","text":"Reading the main file."},
  {"type":"tool_use","name":"Read","input":{"path":"/Users/alice/Code/membot/cmd/membot/main.go"}}
]}}
```

**Timestamp inside a text block (common for user turns):**

```json
{"role":"user","message":{"content":[{"type":"text","text":"<timestamp>Tuesday, May 26, 2026, 10:57 PM (UTC+2)</timestamp>\n<user_query>hello</user_query>"}]}}
```
