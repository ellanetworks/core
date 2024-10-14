-- name: ListDeviceGroupSubscribers :many
SELECT * FROM device_group_subscribers
WHERE device_group_id = ?;

-- name: GetDeviceGroupSubscriber :one
SELECT * FROM device_group_subscribers
WHERE device_group_id = ? AND subscriber_id = ?
LIMIT 1;

-- name: CreateDeviceGroupSubscriber :exec
INSERT INTO device_group_subscribers (
  device_group_id, subscriber_id
) VALUES (
  ?, ?
);

-- name: DeleteDeviceGroupSubscriber :exec
DELETE FROM device_group_subscribers
WHERE device_group_id = ? AND subscriber_id = ?;