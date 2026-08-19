// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import type { APIRequestContext } from "@playwright/test";

export const ADMIN_EMAIL = "e2e-admin@ellanetworks.com";
export const ADMIN_PASSWORD = "E2eAdminPassw0rd!";

async function json<T>(
  request: APIRequestContext,
  method: "get" | "post" | "delete",
  path: string,
  opts: { token?: string; data?: unknown } = {},
): Promise<T> {
  const response = await request[method](path, {
    data: opts.data as never,
    headers: opts.token ? { Authorization: `Bearer ${opts.token}` } : undefined,
  });

  if (!response.ok()) {
    throw new Error(
      `${method.toUpperCase()} ${path} -> ${response.status()} ${await response.text()}`,
    );
  }

  const body = await response.text();
  if (!body) return undefined as T;
  const parsed = JSON.parse(body) as { result?: T };
  return (parsed && "result" in parsed ? parsed.result : parsed) as T;
}

export async function isInitialized(
  request: APIRequestContext,
): Promise<boolean> {
  const status = await json<{ initialized?: boolean }>(
    request,
    "get",
    "/api/v1/status",
  );
  return !!status?.initialized;
}

export async function ensureInitialized(
  request: APIRequestContext,
): Promise<void> {
  if (await isInitialized(request)) return;

  const response = await request.post("/api/v1/init", {
    data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
  });

  if (!response.ok() && !(await isInitialized(request))) {
    throw new Error(
      `could not initialize the core: ${response.status()} ${await response.text()}`,
    );
  }
}

export async function adminToken(request: APIRequestContext): Promise<string> {
  await ensureInitialized(request);

  const login = await json<{ token: string }>(
    request,
    "post",
    "/api/v1/auth/login",
    { data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD } },
  );
  return login.token;
}

export const RoleID = {
  Admin: 1,
  ReadOnly: 2,
  NetworkManager: 3,
} as const;

export async function ensureUser(
  request: APIRequestContext,
  token: string,
  email: string,
  password: string,
  roleId: number,
): Promise<void> {
  const existing = await request.get(
    `/api/v1/users/${encodeURIComponent(email)}`,
    { headers: { Authorization: `Bearer ${token}` } },
  );
  if (existing.ok()) return;

  const response = await request.post("/api/v1/users", {
    headers: { Authorization: `Bearer ${token}` },
    data: { email, password, role_id: roleId },
  });

  if (!response.ok()) {
    const retry = await request.get(
      `/api/v1/users/${encodeURIComponent(email)}`,
      { headers: { Authorization: `Bearer ${token}` } },
    );
    if (!retry.ok()) {
      throw new Error(
        `could not create ${email}: ${response.status()} ${await response.text()}`,
      );
    }
  }
}

export async function deleteSubscriberIfPresent(
  request: APIRequestContext,
  token: string,
  imsi: string,
): Promise<void> {
  await request.delete(`/api/v1/subscribers/${encodeURIComponent(imsi)}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
}
