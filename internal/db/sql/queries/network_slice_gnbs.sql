-- name: ListNetworkSliceGnbs :many
SELECT * FROM network_slice_gnbs
WHERE network_slice_id = ?;

-- name: GetNetworkSliceGnb :one
SELECT * FROM network_slice_gnbs
WHERE network_slice_id = ? AND gnb_id = ?
LIMIT 1;

-- name: CreateNetworkSliceGnb :exec
INSERT INTO network_slice_gnbs (
  network_slice_id, gnb_id
) VALUES (
  ?, ?
);

-- name: DeleteNetworkSliceGnb :exec
DELETE FROM network_slice_gnbs
WHERE network_slice_id = ? AND gnb_id = ?;