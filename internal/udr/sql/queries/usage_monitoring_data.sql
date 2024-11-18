-- name: InsertOrUpdateUsageMonData :exec
INSERT INTO usage_mon_data (ue_id, limit_id, um_level, allowed_usage, reset_time)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (ue_id, limit_id)
DO UPDATE SET 
    um_level = excluded.um_level,
    allowed_usage = excluded.allowed_usage,
    reset_time = excluded.reset_time,
    updated_at = CURRENT_TIMESTAMP;

-- name: GetUsageMonitoringData :one
SELECT ue_id, limit_id, um_level, allowed_usage, reset_time
FROM usage_mon_data
WHERE ue_id = ? AND limit_id = ?;

-- name: GetUsageMonitoringDataByUeID :many
SELECT id, ue_id, limit_id, um_level, allowed_usage, reset_time
FROM usage_mon_data
WHERE ue_id = ?;

-- name: GetUsageMonitoringDataById :one
SELECT id, ue_id, limit_id, um_level, allowed_usage, reset_time
FROM usage_mon_data
WHERE id = ?;

-- name: DeleteUsageMonData :exec
DELETE FROM usage_mon_data
WHERE ue_id = ? AND limit_id = ?;
