-- name: CreateProject :one
INSERT INTO projects (
    canonical_path, slug, name, git_root, remote_url, first_seen_at, last_seen_at
)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpsertProject :one
INSERT INTO projects (
    canonical_path, slug, name, git_root, remote_url, first_seen_at, last_seen_at
)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(slug) DO UPDATE SET
    canonical_path = excluded.canonical_path,
    name = excluded.name,
    git_root = excluded.git_root,
    remote_url = excluded.remote_url,
    last_seen_at = excluded.last_seen_at
RETURNING *;

-- name: GetProject :one
SELECT * FROM projects
WHERE id = ?;

-- name: GetProjectBySlug :one
SELECT * FROM projects
WHERE slug = ?;

-- name: ListProjects :many
SELECT * FROM projects
ORDER BY coalesce(last_seen_at, first_seen_at) DESC, slug;

-- name: UpdateProject :one
UPDATE projects
SET canonical_path = ?,
    slug = ?,
    name = ?,
    git_root = ?,
    remote_url = ?,
    first_seen_at = ?,
    last_seen_at = ?
WHERE id = ?
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects
WHERE id = ?;

-- name: CreateFile :one
INSERT INTO files (project_id, path, normalized_path, kind, basename)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: UpsertFile :one
INSERT INTO files (project_id, path, normalized_path, kind, basename)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(project_id, path) DO UPDATE SET
    normalized_path = excluded.normalized_path,
    kind = excluded.kind,
    basename = excluded.basename
RETURNING *;

-- name: GetFile :one
SELECT * FROM files
WHERE id = ?;

-- name: ListProjectFiles :many
SELECT * FROM files
WHERE project_id = ?
ORDER BY path;

-- name: DeleteFile :exec
DELETE FROM files
WHERE id = ?;
