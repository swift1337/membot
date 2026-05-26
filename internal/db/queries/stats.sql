-- name: GetIndexStats :one
SELECT
    (SELECT count(*) FROM projects) AS project_count,
    (SELECT count(*) FROM conversations) AS conversation_count,
    (SELECT count(*) FROM messages) AS message_count,
    (SELECT count(*) FROM source_files) AS source_file_count,
    (SELECT coalesce(max(indexed_at), '') FROM source_files) AS last_indexed_at;

-- name: ListProjectSummaries :many
SELECT
    coalesce(p.name, p.slug) AS project_name,
    coalesce(p.canonical_path, p.git_root, '') AS project_dir,
    count(c.id) AS chats
FROM projects p
LEFT JOIN conversations c ON c.project_id = p.id
GROUP BY p.id
ORDER BY project_name;
