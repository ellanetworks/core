// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

const b64url = (obj: unknown) =>
  Buffer.from(JSON.stringify(obj))
    .toString("base64")
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");

export function makeToken({
  email = "admin@ellanetworks.com",
  roleId = 1,
  expiresInSec = 3600,
  nowMs = Date.now(),
}: {
  email?: string;
  roleId?: number;
  expiresInSec?: number;
  nowMs?: number;
} = {}) {
  const payload = {
    email,
    role_id: roleId,
    exp: Math.floor(nowMs / 1000) + expiresInSec,
  };
  return `${b64url({ alg: "HS256", typ: "JWT" })}.${b64url(payload)}.sig`;
}
