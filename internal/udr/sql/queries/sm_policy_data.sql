-- name: GetSMPolicyData :one
SELECT * FROM sm_policy_data
WHERE id = ? LIMIT 1;

-- name: GetSMPolicyDataByUeId :one
SELECT * FROM sm_policy_data
WHERE ue_id = ? LIMIT 1;

-- name: CreateSMPolicyData :one
INSERT INTO sm_policy_data (
  ue_id
) VALUES (
  ?
)
RETURNING *;