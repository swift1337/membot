-- name: GetIndexStats :one
SELECT
    (SELECT count(*) FROM projects) AS project_count,
    (SELECT count(*) FROM conversations) AS conversation_count,
    (SELECT count(*) FROM messages) AS message_count,
    (SELECT count(*) FROM source_files) AS source_file_count,
    (SELECT coalesce(max(indexed_at), '') FROM source_files) AS last_indexed_at;

-- name: ListProjectSummaries :many
WITH project_conversations AS (
    SELECT p.id AS project_id, c.id AS conversation_id
    FROM projects p
    JOIN conversations c ON c.project_id = p.id
    UNION
    SELECT p.id AS project_id, c.id AS conversation_id
    FROM projects p
    JOIN tool_calls tc
      ON p.canonical_path IS NOT NULL
     AND (
        tc.working_directory = p.canonical_path
        OR tc.working_directory LIKE p.canonical_path || '/%'
     )
    JOIN messages m ON m.id = tc.message_id
    JOIN conversations c ON c.id = m.conversation_id
    UNION
    SELECT p.id AS project_id, c.id AS conversation_id
    FROM projects p
    JOIN files f
      ON p.canonical_path IS NOT NULL
     AND (
        coalesce(f.normalized_path, f.path) = p.canonical_path
        OR coalesce(f.normalized_path, f.path) LIKE p.canonical_path || '/%'
     )
    JOIN file_mentions fm ON fm.file_id = f.id
    JOIN messages m ON m.id = fm.message_id
    JOIN conversations c ON c.id = m.conversation_id
)
SELECT
    p.slug AS project_slug,
    coalesce(p.name, p.slug) AS project_name,
    coalesce(p.canonical_path, p.git_root, '') AS project_dir,
    count(DISTINCT pc.conversation_id) AS chats
FROM projects p
LEFT JOIN project_conversations pc ON pc.project_id = p.id
GROUP BY p.id
ORDER BY project_name;
