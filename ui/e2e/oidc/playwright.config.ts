// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { defineConfig, devices } from "@playwright/test";

const CORE_PORT = process.env.ELLA_E2E_OIDC_PORT ?? "15444";

export default defineConfig({
  testDir: ".",
  testMatch: /.*\.spec\.ts/,
  outputDir: "../../test-results/oidc",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: process.env.CI
    ? [
        ["github"],
        ["html", { open: "never", outputFolder: "../../playwright-report" }],
      ]
    : "list",
  timeout: 60_000,
  expect: { timeout: 10_000 },
  use: {
    ...devices["Desktop Chrome"],
    baseURL: `https://localhost:${CORE_PORT}`,
    ignoreHTTPSErrors: true,
    launchOptions: {
      args: ["--host-resolver-rules=MAP keycloak 127.0.0.1"],
    },
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
});
