-- name: CreateMemoryItem :one
INSERT INTO memory_items (
    project_id, conversation_id, message_id, kind, title, body,
    confidence, importance, happened_at, evidence_json
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetMemoryItem :one
SELECT * FROM memory_items
WHERE id = ?;

-- name: ListMemoryItems :many
SELECT * FROM memory_items
ORDER BY coalesce(happened_at, created_at) DESC, id DESC
LIMIT ? OFFSET ?;

-- name: ListProjectMemoryItems :many
SELECT * FROM memory_items
WHERE project_id = ?
ORDER BY coalesce(happened_at, created_at) DESC, id DESC
LIMIT ? OFFSET ?;

-- name: ListMemoryItemsByKind :many
SELECT * FROM memory_items
WHERE kind = ?
ORDER BY coalesce(happened_at, created_at) DESC, id DESC
LIMIT ? OFFSET ?;

-- name: DeleteMemoryItem :exec
DELETE FROM memory_items
WHERE id = ?;

-- name: CreateTaskItem :one
INSERT INTO task_items (
    project_id, conversation_id, memory_item_id, title, step_number,
    status, body, evidence_message_id, updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetTaskItem :one
SELECT * FROM task_items
WHERE id = ?;

-- name: ListProjectTasks :many
SELECT * FROM task_items
WHERE project_id = ?
ORDER BY step_number, id;

-- name: ListProjectTasksByStatus :many
SELECT * FROM task_items
WHERE project_id = ? AND status = ?
ORDER BY step_number, id;

-- name: UpdateTaskStatus :one
UPDATE task_items
SET status = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteTaskItem :exec
DELETE FROM task_items
WHERE id = ?;

-- name: CreateEntity :one
INSERT INTO entities (type, value, label, normalized_value, first_seen_at, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpsertEntity :one
INSERT INTO entities (type, value, label, normalized_value, first_seen_at, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(type, normalized_value) DO UPDATE SET
    value = excluded.value,
    label = excluded.label,
    last_seen_at = excluded.last_seen_at
RETURNING *;

-- name: GetEntity :one
SELECT * FROM entities
WHERE id = ?;

-- name: ListEntitiesByType :many
SELECT * FROM entities
WHERE type = ?
ORDER BY last_seen_at DESC, value;

-- name: DeleteEntity :exec
DELETE FROM entities
WHERE id = ?;

-- name: CreateEntityMention :one
INSERT INTO entity_mentions (
    entity_id, message_id, memory_item_id, project_id, context, confidence
)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: ListEntityMentions :many
SELECT * FROM entity_mentions
WHERE entity_id = ?
ORDER BY id;

-- name: DeleteEntityMention :exec
DELETE FROM entity_mentions
WHERE id = ?;

-- name: CreateTopic :one
INSERT INTO topics (name, normalized_name)
VALUES (?, ?)
RETURNING *;

-- name: UpsertTopic :one
INSERT INTO topics (name, normalized_name)
VALUES (?, ?)
ON CONFLICT(normalized_name) DO UPDATE SET
    name = excluded.name
RETURNING *;

-- name: ListTopics :many
SELECT * FROM topics
ORDER BY normalized_name;

-- name: DeleteTopic :exec
DELETE FROM topics
WHERE id = ?;

-- name: LinkConversationTopic :exec
INSERT INTO conversation_topics (conversation_id, topic_id, score)
VALUES (?, ?, ?)
ON CONFLICT(conversation_id, topic_id) DO UPDATE SET
    score = excluded.score;

-- name: ListConversationTopics :many
SELECT t.*, ct.score
FROM conversation_topics ct
JOIN topics t ON t.id = ct.topic_id
WHERE ct.conversation_id = ?
ORDER BY ct.score DESC, t.normalized_name;

-- name: UnlinkConversationTopic :exec
DELETE FROM conversation_topics
WHERE conversation_id = ? AND topic_id = ?;
