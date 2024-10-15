-- name: GetGnb :one
SELECT * FROM gnbs
WHERE id = ? LIMIT 1;

-- name: GetGnbByName :one
SELECT * FROM gnbs
WHERE name = ? LIMIT 1;

-- name: ListGnbs :many
SELECT * FROM gnbs
ORDER BY id;


-- name: CreateGnb :one
INSERT INTO gnbs (
  name, tac, network_slice_id
) VALUES (
  ?, ?, ?
)
RETURNING *;

-- name: DeleteGnb :exec
DELETE FROM gnbs
WHERE id = ?;

-- name: NumGnbs :one
SELECT COUNT(*) FROM gnbs;
