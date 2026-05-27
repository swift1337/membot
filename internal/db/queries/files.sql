-- name: SearchFileContext :many
SELECT
    f.path,
    coalesce(f.normalized_path, f.path) AS normalized_path,
    coalesce(p.name, p.slug, '') AS project_name,
    coalesce(p.canonical_path, p.git_root, '') AS project_dir,
    c.id AS conversation_id,
    coalesce(c.title, '') AS conversation_title,
    count(fm.id) AS mentions,
    max(coalesce(m.created_at, c.started_at, '')) AS last_mentioned_at,
    group_concat(DISTINCT fm.mention_kind) AS mention_kinds,
    coalesce((
        SELECT substr(coalesce(fm2.snippet, ''), 1, 400)
        FROM file_mentions fm2
        JOIN messages m2 ON m2.id = fm2.message_id
        WHERE fm2.file_id = f.id
          AND m2.conversation_id = c.id
          AND coalesce(fm2.snippet, '') != ''
        ORDER BY coalesce(m2.created_at, '') DESC, fm2.id DESC
        LIMIT 1
    ), '') AS snippet
FROM files f
JOIN file_mentions fm ON fm.file_id = f.id
JOIN messages m ON m.id = fm.message_id
JOIN conversations c ON c.id = m.conversation_id
LEFT JOIN projects p ON p.id = f.project_id
WHERE (
  @enable_project = 0
  OR f.project_id = @project_id
  OR (
    @project_path != ''
    AND (
      coalesce(f.normalized_path, f.path) = @project_path
      OR coalesce(f.normalized_path, f.path) LIKE @project_path || '/%'
    )
  )
)
  AND (
    (@basename != '' AND f.basename = @basename)
    OR coalesce(f.normalized_path, f.path) LIKE @path_contains
    OR coalesce(f.normalized_path, f.path) LIKE @path_suffix
  )
GROUP BY f.id, c.id
ORDER BY last_mentioned_at DESC, f.path, c.id
LIMIT @result_limit;
