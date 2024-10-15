-- name: GetNetworkSlice :one
SELECT * FROM network_slices
WHERE id = ? LIMIT 1;

-- name: GetNetworkSliceByName :one
SELECT * FROM network_slices
WHERE name = ? LIMIT 1;

-- name: ListNetworkSlices :many
SELECT * FROM network_slices
ORDER BY id;


-- name: CreateNetworkSlice :one
INSERT INTO network_slices (
  name, sst, sd, site_name, mcc, mnc
) VALUES (
  ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: DeleteNetworkSlice :exec
DELETE FROM network_slices
WHERE id = ?;

-- name: NumNetworkSlices :one
SELECT COUNT(*) FROM network_slices;
