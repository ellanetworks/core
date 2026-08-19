// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { test as setup } from "@playwright/test";
import {
  ADMIN_EMAIL,
  ADMIN_PASSWORD,
  adminToken,
  ensureInitialized,
  ensureUser,
} from "./api";
import { ROLES, ROLE_PASSWORD } from "./roles";

export const SESSION_FILE = "e2e/.auth/admin.json";

setup("authenticate", async ({ request }) => {
  await ensureInitialized(request);

  const response = await request.post("/api/v1/auth/login", {
    data: { email: ADMIN_EMAIL, password: ADMIN_PASSWORD },
  });

  if (!response.ok()) {
    throw new Error(
      `could not authenticate: ${response.status()} ${await response.text()}`,
    );
  }

  await request.storageState({ path: SESSION_FILE });
});

setup("seed non-admin roles", async ({ request, playwright, baseURL }) => {
  const token = await adminToken(request);

  for (const role of Object.values(ROLES)) {
    await ensureUser(request, token, role.email, ROLE_PASSWORD, role.roleId);

    const context = await playwright.request.newContext({ baseURL });
    const response = await context.post("/api/v1/auth/login", {
      data: { email: role.email, password: ROLE_PASSWORD },
    });

    if (!response.ok()) {
      throw new Error(
        `${role.email} could not sign in: ${response.status()} ${await response.text()}`,
      );
    }

    await context.storageState({ path: role.session });
    await context.dispose();
  }
});
