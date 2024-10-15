-- name: ListNetworkSliceDeviceGroups :many
SELECT * FROM network_slice_device_groups
WHERE network_slice_id = ?;

-- name: GetNetworkSliceDeviceGroup :one
SELECT * FROM network_slice_device_groups
WHERE network_slice_id = ? AND device_group_id = ?
LIMIT 1;

-- name: CreateNetworkSliceDeviceGroup :exec
INSERT INTO network_slice_device_groups (
  network_slice_id, device_group_id
) VALUES (
  ?, ?
);

-- name: DeleteNetworkSliceDeviceGroup :exec
DELETE FROM network_slice_device_groups
WHERE network_slice_id = ? AND device_group_id = ?;