-- name: GetUPF :one
SELECT * FROM upfs
WHERE id = ? LIMIT 1;

-- name: GetUPFByName :one
SELECT * FROM upfs
WHERE name = ? LIMIT 1;

-- name: ListUPFs :many
SELECT * FROM upfs
ORDER BY id;

-- name: CreateUPF :one
INSERT INTO upfs (
  name, network_slice_id
) VALUES (
  ?, ?
)
RETURNING *;

-- name: DeleteUPF :exec
DELETE FROM upfs
WHERE id = ?;

-- name: NumUPFs :one
SELECT COUNT(*) FROM upfs;
