-- name: CreateToolCall :one
INSERT INTO tool_calls (
    message_id, block_id, tool_name, arguments_json, working_directory,
    status, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetToolCall :one
SELECT * FROM tool_calls
WHERE id = ?;

-- name: ListMessageToolCalls :many
SELECT * FROM tool_calls
WHERE message_id = ?
ORDER BY id;

-- name: DeleteToolCall :exec
DELETE FROM tool_calls
WHERE id = ?;

-- name: DeleteToolCallFileMentionsForMessage :exec
DELETE FROM file_mentions
WHERE tool_call_id IN (
    SELECT tc.id FROM tool_calls tc WHERE tc.message_id = ?
);

-- name: DeleteToolCallsForMessage :exec
DELETE FROM tool_calls
WHERE message_id = ?;

-- name: CreateFileMention :one
INSERT INTO file_mentions (
    file_id, message_id, tool_call_id, mention_kind, line_start, line_end, snippet
)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListFileMentions :many
SELECT * FROM file_mentions
WHERE file_id = ?
ORDER BY id;

-- name: ListMessageFileMentions :many
SELECT * FROM file_mentions
WHERE message_id = ?
ORDER BY id;

-- name: DeleteFileMentionsForMessage :exec
DELETE FROM file_mentions
WHERE message_id = ?;

-- name: DeleteFileMention :exec
DELETE FROM file_mentions
WHERE id = ?;

-- name: ListRelatedFilesForConversations :many
SELECT
    f.path,
    coalesce(p.name, p.slug, '') AS project_name,
    count(*) AS mentions
FROM file_mentions fm
JOIN files f ON f.id = fm.file_id
LEFT JOIN projects p ON p.id = f.project_id
JOIN messages m ON m.id = fm.message_id
WHERE m.conversation_id IN (sqlc.slice(conversation_ids))
GROUP BY f.id
ORDER BY mentions DESC, f.path
LIMIT ?;

-- name: ListToolCallsForConversations :many
SELECT
    tc.id,
    tc.tool_name,
    coalesce(tc.status, '') AS status,
    coalesce(tc.created_at, '') AS created_at,
    tc.message_id,
    coalesce(tc.arguments_json, '') AS arguments_json,
    coalesce(tc.working_directory, '') AS working_directory
FROM tool_calls tc
JOIN messages m ON m.id = tc.message_id
WHERE m.conversation_id IN (sqlc.slice(conversation_ids))
ORDER BY coalesce(tc.created_at, '') DESC, tc.id DESC
LIMIT ?;
