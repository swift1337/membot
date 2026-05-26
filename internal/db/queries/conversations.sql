-- name: CreateConversation :one
INSERT INTO conversations (
    source_id, project_id, source_file_id, external_id, parent_external_id,
    title, started_at, ended_at, message_count, is_subagent, raw_path
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpsertConversation :one
INSERT INTO conversations (
    source_id, project_id, source_file_id, external_id, parent_external_id,
    title, started_at, ended_at, message_count, is_subagent, raw_path
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(source_id, external_id) DO UPDATE SET
    project_id = excluded.project_id,
    source_file_id = excluded.source_file_id,
    parent_external_id = excluded.parent_external_id,
    title = excluded.title,
    started_at = excluded.started_at,
    ended_at = excluded.ended_at,
    message_count = excluded.message_count,
    is_subagent = excluded.is_subagent,
    raw_path = excluded.raw_path
RETURNING *;

-- name: GetConversation :one
SELECT * FROM conversations
WHERE id = ?;

-- name: GetConversationByExternalID :one
SELECT * FROM conversations
WHERE source_id = ? AND external_id = ?;

-- name: ListConversations :many
SELECT * FROM conversations
ORDER BY coalesce(started_at, ended_at) DESC, id DESC
LIMIT ? OFFSET ?;

-- name: ListProjectConversations :many
SELECT * FROM conversations
WHERE project_id = ?
ORDER BY coalesce(started_at, ended_at) DESC, id DESC
LIMIT ? OFFSET ?;

-- name: DeleteConversation :exec
DELETE FROM conversations
WHERE id = ?;

-- name: CreateMessage :one
INSERT INTO messages (
    conversation_id, source_file_id, role, seq, created_at, text, raw_json, raw_line, content_hash
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpsertMessage :one
INSERT INTO messages (
    conversation_id, source_file_id, role, seq, created_at, text, raw_json, raw_line, content_hash
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(source_file_id, raw_line) DO UPDATE SET
    conversation_id = excluded.conversation_id,
    role = excluded.role,
    seq = excluded.seq,
    created_at = excluded.created_at,
    text = excluded.text,
    raw_json = excluded.raw_json,
    content_hash = excluded.content_hash
RETURNING *;

-- name: GetMessage :one
SELECT * FROM messages
WHERE id = ?;

-- name: GetMessageByRawLine :one
SELECT * FROM messages
WHERE source_file_id = ? AND raw_line = ?;

-- name: ListConversationMessages :many
SELECT * FROM messages
WHERE conversation_id = ?
ORDER BY seq;

-- name: DeleteMessage :exec
DELETE FROM messages
WHERE id = ?;

-- name: CreateMessageBlock :one
INSERT INTO message_blocks (message_id, seq, type, text, raw_json)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpsertMessageBlock :one
INSERT INTO message_blocks (message_id, seq, type, text, raw_json)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(message_id, seq) DO UPDATE SET
    type = excluded.type,
    text = excluded.text,
    raw_json = excluded.raw_json
RETURNING *;

-- name: ListMessageBlocks :many
SELECT * FROM message_blocks
WHERE message_id = ?
ORDER BY seq;

-- name: DeleteMessageBlock :exec
DELETE FROM message_blocks
WHERE id = ?;
