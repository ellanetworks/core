-- name: GetIPPoolByCIDR :one
SELECT * FROM IPPool
WHERE cidr = ?;

-- name: CreateIPPool :one
INSERT INTO IPPool (cidr)
VALUES (?)
RETURNING id;

-- name: AllocateIP :exec
INSERT INTO AllocatedIP (
  imsi, ip_address, pool_id
) VALUES (
  ?, ?, ?
);

-- name: ReleaseIP :exec
DELETE FROM AllocatedIP
WHERE imsi = ?;

-- name: FindAvailableIP :one
SELECT ip_address FROM AllocatedIP
WHERE ip_address = ? AND pool_id = ?;

-- name: GetIPPoolCIDR :one
SELECT cidr FROM IPPool
WHERE id = ?;

-- name: GetAllocatedIPByIMSI :one
SELECT * FROM AllocatedIP
WHERE imsi = ?;
