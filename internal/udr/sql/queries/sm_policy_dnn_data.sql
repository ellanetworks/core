-- name: GetSmPolicyDnnDataBySnssaiId :many
SELECT id, sm_policy_snssai_data_id, dnn
FROM sm_policy_dnn_data
WHERE sm_policy_snssai_data_id = ?;
