-- name: InsertUsageMonDataScope :exec
INSERT INTO usage_mon_data_scopes (usage_mon_data_id, snssai_sd, snssai_sst, dnn)
VALUES (?, ?, ?, ?);

-- name: GetUsageMonDataID :one
SELECT id
FROM usage_mon_data
WHERE ue_id = ? AND limit_id = ?;

-- name: DeleteUsageMonDataScopesById :exec
DELETE FROM usage_mon_data_scopes
WHERE usage_mon_data_id = ?;
