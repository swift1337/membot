# Codex transcript indexing

This document explains how Codex stores assistant history on disk and how
`membot` discovers, parses, and indexes it. It is written for agents working on
the indexer, query layer, or MCP tools.

## Where Codex stores data

Codex keeps durable CLI and Desktop transcripts under the user's home directory:

```text
~/.codex/
├── sessions/                         # indexed
│   └── YYYY/MM/DD/
│       └── rollout-<created>-<thread id>.jsonl
├── state_5.sqlite                    # optional metadata enrichment
├── session_index.jsonl               # optional metadata enrichment
└── ...                               # settings/cache/runtime data ignored
```

**Default root:** `~/.codex` (`codex.DefaultRoot()`).

The `membot index codex` command uses this default root. To index a custom
Codex root, use `membot index all --codex-root /path/to/.codex`.

`~/Library/Application Support/Codex` is Desktop UI/runtime profile state. It
contains app diagnostics, profile data, caches, and telemetry-related state, not
the durable chat transcript source. The indexer intentionally does not read it.

## Transcript file layout

Transcripts live below `sessions/YYYY/MM/DD/` as JSONL files:

```text
sessions/
└── 2026/06/04/
    └── rollout-2026-06-04T10-20-30Z-<thread id>.jsonl
```

Conventions:

- **Conversation ID** = thread id from JSONL metadata, otherwise the rollout
  filename suffix after the created timestamp.
- Date folders are chronological storage only. They are not project folders.
- **Project** = the thread working directory (`cwd`) from metadata.

## Metadata enrichment

The indexer builds conversation metadata in this order:

1. `state_5.sqlite.threads` by thread id: `title`, `cwd`.
2. `session_index.jsonl` by thread id: `thread_name`/`title`, `cwd`.
3. JSONL metadata events: `session_meta.payload.cwd`,
   `turn_context.payload.cwd`, and thread/session ids.
4. First user message as a title fallback.
5. Source file modification time as a timestamp fallback.

Project mapping:

| Codex concept | DB mapping |
| --- | --- |
| rollout JSONL path | `source_files.path` |
| thread id | `conversations.external_id` |
| thread title / first prompt | `conversations.title` |
| event timestamps | `conversations.started_at`, `conversations.ended_at` |
| `cwd` | `projects.canonical_path` |
| slug from `cwd` | `projects.slug` |
| basename of `cwd` | `projects.name` |

## Parsed content

The parser (`codex-jsonl-v1`) keeps human-useful transcript content:

- User prompts from `event_msg` payloads with `type=user_message`.
- User and assistant `response_item` messages where `item.type=message`.
- Assistant-visible response text from message content blocks.
- Concise `response_item` `function_call` records in `tool_calls`.

The parser skips prompt/runtime noise:

- `session_meta.payload.base_instructions`.
- `response_item` messages with role `developer` or `system`.
- `turn_context` as searchable text.
- encrypted reasoning, token counters, telemetry, Sentry/app events, and other
  runtime-only events.
- `event_msg.agent_message`, because it can mirror assistant
  `response_item.message` records.

Raw JSON is preserved on indexed messages and blocks where useful, but
system/developer/base instructions are not made searchable.

## How membot indexes transcripts

Entry point: `codex.Index(ctx, store, Options{Root})`.

CLI: `membot index codex [--reindex]`.

```text
discover(root/sessions)
  -> walk sessions/*/*/*/rollout-*.jsonl
  -> read optional state_5.sqlite and session_index.jsonl metadata
  -> for each file: indexTranscript (skip if unchanged)
       -> parse JSONL lines and metadata
       -> map cwd to project
       -> upsert source_file and conversation
       -> upsert user/assistant messages and message blocks
       -> create tool_calls for function_call items
       -> record parse_errors for bad lines
```

### Incremental indexing

A transcript is skipped when an existing `source_files` row matches:

- Same `source_id` + absolute path.
- Same `size_bytes`, `mtime_unix`, `sha256`.
- Same `parser_version` (`codex-jsonl-v1`).

`--reindex` deletes the entire SQLite database and rebuilds from scratch.

## Code map

| File | Responsibility |
| --- | --- |
| `internal/indexer/codex/indexer.go` | Discovery, metadata enrichment, parsing, indexing |
| `internal/indexer/codex/indexer_test.go` | Rollout ID, metadata, exclusion, and tool-call tests |

Key constants in `indexer.go`:

```go
sourceKind    = "codex"
parserVersion = "codex-jsonl-v1"
```
