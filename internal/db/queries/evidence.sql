-- name: CreateArtifact :one
INSERT INTO artifacts (source_file_id, conversation_id, path, kind, text, sha256, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetArtifact :one
SELECT * FROM artifacts
WHERE id = ?;

-- name: ListConversationArtifacts :many
SELECT * FROM artifacts
WHERE conversation_id = ?
ORDER BY created_at, id;

-- name: DeleteArtifact :exec
DELETE FROM artifacts
WHERE id = ?;

-- name: CreateToolCall :one
INSERT INTO tool_calls (
    message_id, block_id, tool_name, arguments_json, working_directory,
    status, output_artifact_id, created_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
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

-- name: DeleteFileMention :exec
DELETE FROM file_mentions
WHERE id = ?;

-- name: CreatePatch :one
INSERT INTO patches (
    tool_call_id, message_id, file_id, patch_kind, raw_patch,
    added_lines, removed_lines, parsed_ok
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListMessagePatches :many
SELECT * FROM patches
WHERE message_id = ?
ORDER BY id;

-- name: ListFilePatches :many
SELECT * FROM patches
WHERE file_id = ?
ORDER BY id;

-- name: DeletePatch :exec
DELETE FROM patches
WHERE id = ?;
