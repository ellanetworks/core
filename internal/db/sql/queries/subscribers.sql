-- name: GetSubscriber :one
SELECT * FROM subscribers
WHERE id = ? LIMIT 1;

-- name: GetSubscriberByImsi :one
SELECT * FROM subscribers
WHERE imsi = ? LIMIT 1;

-- name: ListSubscribers :many
SELECT * FROM subscribers
ORDER BY imsi;

-- name: CreateSubscriber :one
INSERT INTO subscribers (
  imsi, plmn_id, opc, key, sequence_number
) VALUES (
  ?, ?, ?, ?, ?
)
RETURNING *;

-- name: UpdateSubscriber :exec
UPDATE subscribers
set imsi = ?, plmn_id = ?, opc = ?, key = ?, sequence_number = ?
WHERE id = ?;

-- name: DeleteSubscriber :exec
DELETE FROM subscribers
WHERE id = ?;

-- name: NumSubscribers :one
SELECT COUNT(*) FROM subscribers;
