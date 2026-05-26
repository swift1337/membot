-- name: GetIndexStats :one
SELECT
    (SELECT count(*) FROM projects) AS project_count,
    (SELECT count(*) FROM conversations) AS conversation_count,
    (SELECT count(*) FROM messages) AS message_count,
    (SELECT count(*) FROM source_files) AS source_file_count,
    (SELECT coalesce(max(indexed_at), '') FROM source_files) AS last_indexed_at;
