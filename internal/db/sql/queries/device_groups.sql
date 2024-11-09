-- name: GetDeviceGroup :one
SELECT * FROM device_groups
WHERE id = ? LIMIT 1;

-- name: GetDeviceGroupByName :one
SELECT * FROM device_groups
WHERE name = ? LIMIT 1;

-- name: ListDeviceGroups :many
SELECT * FROM device_groups
ORDER BY id;

-- name: ListDeviceGroupsByNetworkSliceId :many
SELECT * FROM device_groups
WHERE network_slice_id = ?
ORDER BY id;

-- name: CreateDeviceGroup :one
INSERT INTO device_groups (
  name, site_info, ip_domain_name, dnn, ue_ip_pool_id, dns_primary, mtu, dnn_mbr_uplink, dnn_mbr_downlink, traffic_class_name, traffic_class_arp, traffic_class_pdb, traffic_class_pelr, traffic_class_qci, network_slice_id
) VALUES (
  ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
)
RETURNING *;

-- name: DeleteDeviceGroup :exec
DELETE FROM device_groups
WHERE id = ?;

-- name: NumDeviceGroups :one
SELECT COUNT(*) FROM device_groups;
