-- name: SearchMessageHits :many
SELECT
    'message' AS result_type,
    bm25(message_fts) AS score,
    coalesce(p.name, p.slug, '') AS project_name,
    coalesce(p.canonical_path, p.git_root, '') AS project_dir,
    c.id AS conversation_id,
    coalesce(c.title, '') AS conversation_title,
    m.id AS entity_id,
    m.role AS role,
    coalesce(m.created_at, c.started_at, strftime('%Y-%m-%dT%H:%M:%SZ', sf.mtime_unix, 'unixepoch'), sf.indexed_at, '') AS created_at,
    coalesce(substr(m.text, 1, 400), '') AS snippet
FROM message_fts(@fts_query) f
JOIN messages m ON m.id = f.rowid
JOIN conversations c ON c.id = m.conversation_id
LEFT JOIN projects p ON p.id = c.project_id
LEFT JOIN source_files sf ON sf.id = m.source_file_id
WHERE (
    @enable_project = 0
    OR c.project_id = @project_id
    OR (
        @project_path != ''
        AND EXISTS (
            SELECT 1
            FROM tool_calls tc
            JOIN messages tm ON tm.id = tc.message_id
            WHERE tm.conversation_id = c.id
              AND (
                tc.working_directory = @project_path
                OR tc.working_directory LIKE @project_path || '/%'
              )
        )
    )
    OR (
        @project_path != ''
        AND EXISTS (
            SELECT 1
            FROM file_mentions fm
            JOIN files fp ON fp.id = fm.file_id
            JOIN messages fm_msg ON fm_msg.id = fm.message_id
            WHERE fm_msg.conversation_id = c.id
              AND (
                coalesce(fp.normalized_path, fp.path) = @project_path
                OR coalesce(fp.normalized_path, fp.path) LIKE @project_path || '/%'
              )
        )
    )
)
  AND (@enable_since = 0 OR coalesce(m.created_at, c.started_at, strftime('%Y-%m-%dT%H:%M:%SZ', sf.mtime_unix, 'unixepoch'), sf.indexed_at, '') >= @since)
ORDER BY score
LIMIT @result_limit;

-- name: SearchMemoryHits :many
SELECT
    'memory' AS result_type,
    bm25(memory_fts) AS score,
    coalesce(p.name, p.slug, '') AS project_name,
    coalesce(p.canonical_path, p.git_root, '') AS project_dir,
    coalesce(mi.conversation_id, 0) AS conversation_id,
    '' AS conversation_title,
    mi.id AS entity_id,
    mi.kind AS role,
    coalesce(mi.happened_at, mi.created_at, '') AS created_at,
    coalesce(substr(mi.title || ' ' || mi.body, 1, 400), '') AS snippet
FROM memory_fts(@fts_query) f
JOIN memory_items mi ON mi.id = f.rowid
LEFT JOIN projects p ON p.id = mi.project_id
LEFT JOIN conversations c ON c.id = mi.conversation_id
WHERE (
    @enable_project = 0
    OR mi.project_id = @project_id
    OR c.project_id = @project_id
    OR (
        @project_path != ''
        AND c.id IS NOT NULL
        AND EXISTS (
            SELECT 1
            FROM tool_calls tc
            JOIN messages tm ON tm.id = tc.message_id
            WHERE tm.conversation_id = c.id
              AND (
                tc.working_directory = @project_path
                OR tc.working_directory LIKE @project_path || '/%'
              )
        )
    )
    OR (
        @project_path != ''
        AND c.id IS NOT NULL
        AND EXISTS (
            SELECT 1
            FROM file_mentions fm
            JOIN files fp ON fp.id = fm.file_id
            JOIN messages fm_msg ON fm_msg.id = fm.message_id
            WHERE fm_msg.conversation_id = c.id
              AND (
                coalesce(fp.normalized_path, fp.path) = @project_path
                OR coalesce(fp.normalized_path, fp.path) LIKE @project_path || '/%'
              )
        )
    )
)
  AND (@enable_since = 0 OR coalesce(mi.happened_at, mi.created_at, '') >= @since)
ORDER BY score
LIMIT @result_limit;

-- name: SearchArtifactHits :many
SELECT
    'artifact' AS result_type,
    bm25(artifact_fts) AS score,
    coalesce(p.name, p.slug, '') AS project_name,
    coalesce(p.canonical_path, p.git_root, '') AS project_dir,
    coalesce(a.conversation_id, 0) AS conversation_id,
    '' AS conversation_title,
    a.id AS entity_id,
    a.kind AS role,
    coalesce(a.created_at, '') AS created_at,
    coalesce(substr(a.path || ' ' || coalesce(a.text, ''), 1, 400), '') AS snippet
FROM artifact_fts(@fts_query) f
JOIN artifacts a ON a.id = f.rowid
LEFT JOIN conversations c ON c.id = a.conversation_id
LEFT JOIN projects p ON p.id = c.project_id
WHERE (
    @enable_project = 0
    OR c.project_id = @project_id
    OR (
        @project_path != ''
        AND EXISTS (
            SELECT 1
            FROM tool_calls tc
            JOIN messages tm ON tm.id = tc.message_id
            WHERE tm.conversation_id = c.id
              AND (
                tc.working_directory = @project_path
                OR tc.working_directory LIKE @project_path || '/%'
              )
        )
    )
    OR (
        @project_path != ''
        AND EXISTS (
            SELECT 1
            FROM file_mentions fm
            JOIN files fp ON fp.id = fm.file_id
            JOIN messages fm_msg ON fm_msg.id = fm.message_id
            WHERE fm_msg.conversation_id = c.id
              AND (
                coalesce(fp.normalized_path, fp.path) = @project_path
                OR coalesce(fp.normalized_path, fp.path) LIKE @project_path || '/%'
              )
        )
    )
)
  AND (@enable_since = 0 OR coalesce(a.created_at, '') >= @since)
ORDER BY score
LIMIT @result_limit;
