-- name: GetRadio :one
SELECT * FROM radios
WHERE id = ? LIMIT 1;

-- name: GetRadioByName :one
SELECT * FROM radios
WHERE name = ? LIMIT 1;

-- name: ListRadios :many
SELECT * FROM radios
ORDER BY id;


-- name: CreateRadio :one
INSERT INTO radios (
  name, tac, network_slice_id
) VALUES (
  ?, ?, ?
)
RETURNING *;

-- name: DeleteRadio :exec
DELETE FROM radios
WHERE id = ?;

-- name: NumRadios :one
SELECT COUNT(*) FROM radios;
