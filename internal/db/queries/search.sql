-- name: SearchMessages :many
SELECT m.*
FROM message_fts(?) f
JOIN messages m ON m.id = f.rowid
ORDER BY rank
LIMIT ?;

-- name: SearchMemoryItems :many
SELECT mi.*
FROM memory_fts(?) f
JOIN memory_items mi ON mi.id = f.rowid
ORDER BY rank
LIMIT ?;

-- name: SearchArtifacts :many
SELECT a.*
FROM artifact_fts(?) f
JOIN artifacts a ON a.id = f.rowid
ORDER BY rank
LIMIT ?;
