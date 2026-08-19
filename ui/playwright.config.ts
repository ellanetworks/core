// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { defineConfig, devices } from "@playwright/test";

const E2E_PORT = process.env.ELLA_E2E_PORT ?? "5102";
const DEFAULT_URL = `http://localhost:${E2E_PORT}`;
const baseURL = process.env.ELLA_E2E_BASE_URL ?? DEFAULT_URL;
const SESSION_FILE = "e2e/.auth/admin.json";

export default defineConfig({
  testDir: "./e2e",
  globalSetup: "./e2e/global-setup.ts",
  globalTeardown: "./e2e/global-teardown.ts",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  timeout: 30_000,
  expect: { timeout: 10_000 },
  use: {
    ...devices["Desktop Chrome"],
    baseURL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    ignoreHTTPSErrors: true,
  },
  projects: [
    {
      name: "setup",
      testMatch: /auth\.setup\.ts/,
    },
    {
      name: "unauthenticated",
      testDir: "./e2e/unauthenticated",
    },
    {
      name: "authenticated",
      testDir: "./e2e/authenticated",
      dependencies: ["setup"],
      use: { storageState: SESSION_FILE },
    },
  ],
  ...(!process.env.ELLA_E2E_BASE_URL && {
    webServer: {
      command:
        "npm run build && " +
        "mkdir -p e2e/.build && (cd .. && go build -o ui/e2e/.build/core ./cmd/core) && " +
        "docker compose -f e2e/compose.yaml -f e2e/compose.local.yaml down -v && " +
        "docker compose -f e2e/compose.yaml -f e2e/compose.local.yaml up",
      env: { ELLA_E2E_PORT: E2E_PORT },
      url: `${DEFAULT_URL}/api/v1/status`,
      reuseExistingServer: false,
      timeout: 120_000,
      stdout: "pipe",
      stderr: "pipe",
    },
  }),
});
