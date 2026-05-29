# Claude Code transcript indexing

This document explains how Claude Code stores assistant history on disk and how
`membot` discovers, parses, and scrapes it. It covers the on-disk layout and the
parsing rules only — not the CLI or the database.

## Where Claude Code stores data

Claude Code keeps per-project transcripts under the user's home directory. The
indexer's **root** is `~/.claude` (`claude.DefaultRoot()`) and it reads the
`projects/` subdirectory under it:

```
~/.claude/
├── projects/                           # ← scraped
│   ├── -Users-alice-Code-myapp/        # project slug (see below)
│   │   ├── <session-uuid>.jsonl        # parent session transcript
│   │   ├── <session-uuid>.jsonl
│   │   ├── memory/                     # ignored (not .jsonl)
│   │   ├── sessions-index.json         # ignored (not .jsonl)
│   │   └── subagents/
│   │       └── <subagent-uuid>.jsonl   # spawned subagent run
│   └── -Users-alice-Code-other-repo/
│       └── <session-uuid>.jsonl
├── memory/                             # ignored
├── sessions/                           # ignored
├── history.jsonl                       # ignored (outside projects/)
├── settings.json                       # ignored
└── ...                                 # backups, cache, plugins, etc. — ignored
```

Only `~/.claude/projects/**/*.jsonl` is read. Everything else under `~/.claude`
(and any non-`.jsonl` file inside `projects/`, such as `sessions-index.json` or
`memory/MEMORY.md`) is ignored.

### Project slugs

Each subdirectory of `projects/` is a **project slug**. Claude Code encodes the
workspace path in the slug by replacing path separators with hyphens, including a
**leading** hyphen for the root `/`:

| Slug | Inferred canonical path |
|------|-------------------------|
| `-Users-alice-Code-membot` | `/Users/alice/Code/membot` |
| `-Volumes-External-myproj` | `/Volumes/External/myproj` |

When scraping, the leading `-` is stripped to form the stored slug
(`strings.TrimPrefix(name, "-")`). The canonical path is reconstructed only when
the trimmed name starts with `Users-` or `Volumes-`: hyphens are replaced with
the OS path separator and a leading separator is prepended
(`inferCanonicalPath`). Slugs that do not match this heuristic still become
projects but get no canonical path; their display name falls back to the slug.

**Every** subdirectory of `projects/` is treated as a project — there is no
transcript-presence or prefix gate, and no `.code-workspace` parsing (Claude Code
has no multi-repo workspace concept here).

### Transcript file layout

Transcripts are **JSONL** files (one JSON object per line). Each project
directory is walked **recursively** and every `*.jsonl` file is ingested:

```
projects/-Users-alice-Code-myapp/
├── 185b46c7-....jsonl                 # parent session
└── subagents/
    └── fc5c473c-....jsonl             # subagent run
```

Conventions derived from the path:

- **Conversation ID** = basename of the `.jsonl` file without extension (a UUID).
- **Parent link** = the path component immediately before a `subagents/`
  directory, when present (`parentExternalID`).
- **Subagent flag** = the path contains `/subagents/` (`isSubagentPath`).

## Transcript line format

Each non-empty line is a JSON object. The parser (`claude-jsonl-v1`) decodes:

```json
{
  "type": "assistant",
  "uuid": "...",
  "parentUuid": "...",
  "isSidechain": false,
  "agentId": "...",
  "sessionId": "...",
  "cwd": "/Users/alice/Code/myapp",
  "timestamp": "2026-05-26T20:57:00.123Z",
  "message": {
    "role": "assistant",
    "content": "... or [...]"
  }
}
```

A line is **kept** when `message.role` is set, or when the top-level `type` is
`"user"` or `"assistant"`. Lines that are neither (summaries, system events) are
silently skipped — they are not parse errors. The effective role is
`message.role`, falling back to `type`, then `"unknown"`.

### Message content shapes

`message.content` is either:

1. **A plain string** — treated as a single `text` block.
2. **An array of blocks** — each block has at least `type` and often `text`:

```json
{
  "type": "assistant",
  "message": {
    "role": "assistant",
    "content": [
      {"type": "text", "text": "I'll look that up."},
      {"type": "tool_use", "name": "Read", "input": {"file_path": "..."}}
    ]
  }
}
```

Common block types seen in transcripts:

| `type` | Typical fields | Scraped for |
|--------|----------------|-------------|
| `text` | `text` | Searchable message text |
| `tool_use` | `name`, `input` | Tool call + file paths from the input |
| `tool_result` | `content` | Flattened text from nested content |

Block text resolution (`parseContent`):

- `text` is used directly when present.
- Otherwise, if a nested `content` field exists, it is flattened — either a
  string or a nested block array joined with `\n\n` (`contentText`).
- Otherwise, for `tool_use` blocks with no text, the tool `name` is used so the
  call is still searchable.

The searchable **message text** is every block's `text` joined with `\n\n`.

### Timestamps

The parser reads the top-level `timestamp` field only and accepts:

- RFC3339Nano (for example `2026-05-26T20:57:00.123Z`)
- RFC3339 (for example `2026-05-26T20:57:00Z`)

Timestamps are normalized to UTC. A line with no parseable timestamp falls back
to the source file's modification time. Conversation start/end are the first and
last resolved line timestamps (`conversationTimes`).

## How membot scrapes a transcript

```
discover(root/projects)
  → list project dirs (every subdirectory is a project)
  → walk <project>/**/*.jsonl
  → for each file: parse JSONL lines
       → keep user/assistant lines, flatten blocks to message text
       → conversation id / parent / subagent flag from the file path
       → extract file mentions from message text
       → extract tool calls + file paths from tool_use blocks
       → bad lines are recorded as parse errors, not fatal
```

### File mention extraction (from message text)

After a message's text is flattened, `extractFileMentions(text)` scans for three
patterns:

| Pattern | Example | `mention_kind` |
|---------|---------|----------------|
| Code citation fence | `` ```247:265:/path/to/file.go `` | `code_ref` (+ line range) |
| Inline backticks | `` `src/main.go` `` | `inline_code` |
| At-path reference | `@sandbox/plan.md` | `at_ref` |

Candidates are dropped when they contain no `/`, contain `://` (URLs), contain
whitespace, or normalize to `.` or `/` only. Paths are cleaned (trim trailing
punctuation, strip leading `@`, remove `123:456:` line prefixes) and normalized
with `filepath.ToSlash(filepath.Clean(path))`. Mentions are de-duplicated per
message by normalized path.

### Tool call extraction (from `tool_use` blocks)

Each `tool_use` block yields a tool call, and file paths are pulled from the tool
input:

| Tool(s) | Source field | `mention_kind` |
|---------|--------------|----------------|
| Read | `file_path` or `path` | `tool_read` |
| Write, Edit, MultiEdit, NotebookEdit | `file_path` or `path` | `tool_write` |
| Glob, Grep | `path` or `target_directory` | `tool_read` |
| ApplyPatch | patch headers (`*** Update/Add/Delete File: ...`) | `tool_patch` |
| Bash | — | working directory only, no file mention |

ApplyPatch arguments are compacted to `{"files":[...]}`; other tools keep the raw
`input` JSON. Bash records a working directory from the input's
`working_directory`, falling back to the line-level `cwd`, and creates no file
mention.

### Parse errors

Malformed JSON lines are **not** fatal. They are recorded with line number, error
text, and a SHA-256 hash of the raw line. Valid lines in the same file are still
scraped.

## Code map

| File | Responsibility |
|------|----------------|
| `internal/indexer/claude/indexer.go` | Discovery, parsing, file mentions, tool calls |
| `internal/indexer/claude/indexer_test.go` | Timestamp, mention, and tool parsing tests |

Key constants in `indexer.go`:

```go
sourceKind    = "claude"
parserVersion = "claude-jsonl-v1"
```

Bump `parserVersion` when parsing logic changes so existing files are re-read.

## Example transcript snippets

**User message (plain string content):**

```json
{"type":"user","message":{"role":"user","content":"Add MCP support"}}
```

**Assistant with text + tool call:**

```json
{"type":"assistant","timestamp":"2026-05-26T20:57:00Z","message":{"role":"assistant","content":[
  {"type":"text","text":"Reading the main file."},
  {"type":"tool_use","name":"Read","input":{"file_path":"/Users/alice/Code/myapp/cmd/main.go"}}
]}}
```

**Tool result line:**

```json
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","content":"package main..."}]}}
```
