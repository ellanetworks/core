// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { test as setup } from "@playwright/test";
import { ADMIN_EMAIL, ADMIN_PASSWORD, ensureInitialized } from "./api";

export const SESSION_FILE = "e2e/.auth/admin.json";

// Authenticating over HTTP rather than through the form keeps this independent
// of the login UI, which the unauthenticated project already covers. The server
// sets an HttpOnly session cookie, so persisting storage state is enough for the
// app to obtain an access token on load.
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
