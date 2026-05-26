-- name: CreateSource :one
INSERT INTO sources (kind, root_path, display_name)
VALUES (?, ?, ?)
RETURNING *;

-- name: UpsertSource :one
INSERT INTO sources (kind, root_path, display_name)
VALUES (?, ?, ?)
ON CONFLICT(kind, root_path) DO UPDATE SET
    display_name = excluded.display_name,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: GetSource :one
SELECT * FROM sources
WHERE id = ?;

-- name: GetSourceByKindRoot :one
SELECT * FROM sources
WHERE kind = ? AND root_path = ?;

-- name: ListSources :many
SELECT * FROM sources
ORDER BY kind, root_path;

-- name: UpdateSource :one
UPDATE sources
SET kind = ?, root_path = ?, display_name = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?
RETURNING *;

-- name: DeleteSource :exec
DELETE FROM sources
WHERE id = ?;

-- name: CreateSourceFile :one
INSERT INTO source_files (
    source_id, project_id, path, kind, size_bytes, mtime_unix, sha256, indexed_at, parser_version
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpsertSourceFile :one
INSERT INTO source_files (
    source_id, project_id, path, kind, size_bytes, mtime_unix, sha256, indexed_at, parser_version
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(source_id, path) DO UPDATE SET
    project_id = excluded.project_id,
    kind = excluded.kind,
    size_bytes = excluded.size_bytes,
    mtime_unix = excluded.mtime_unix,
    sha256 = excluded.sha256,
    indexed_at = excluded.indexed_at,
    parser_version = excluded.parser_version
RETURNING *;

-- name: GetSourceFile :one
SELECT * FROM source_files
WHERE id = ?;

-- name: GetSourceFileByPath :one
SELECT * FROM source_files
WHERE source_id = ? AND path = ?;

-- name: ListSourceFiles :many
SELECT * FROM source_files
WHERE source_id = ?
ORDER BY path;

-- name: DeleteSourceFile :exec
DELETE FROM source_files
WHERE id = ?;

-- name: CreateParseError :one
INSERT INTO parse_errors (source_file_id, raw_line, parser_version, error, raw_hash)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: ListParseErrorsForSourceFile :many
SELECT * FROM parse_errors
WHERE source_file_id = ?
ORDER BY raw_line, id;

-- name: DeleteParseError :exec
DELETE FROM parse_errors
WHERE id = ?;
