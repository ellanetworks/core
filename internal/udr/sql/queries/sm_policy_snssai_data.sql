-- name: InsertOrUpdateSmPolicySnssaiData :exec
INSERT INTO sm_policy_snssai_data (sm_policy_data_id, snssai_sd, snssai_sst)
VALUES (?, ?, ?)
ON CONFLICT (sm_policy_data_id, snssai_sd, snssai_sst)
DO NOTHING;

-- name: GetSmPolicySnssaiDataByPolicyId :many
SELECT id, sm_policy_data_id, snssai_sd, snssai_sst
FROM sm_policy_snssai_data
WHERE sm_policy_data_id = ?;
