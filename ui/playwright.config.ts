// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { defineConfig, devices } from "@playwright/test";

// Deliberately not 5002: these specs create and delete real records, so the
// default must never land on a development core someone is already running.
const E2E_PORT = process.env.ELLA_E2E_PORT ?? "5102";
const DEFAULT_URL = `http://localhost:${E2E_PORT}`;
const baseURL = process.env.ELLA_E2E_BASE_URL ?? DEFAULT_URL;
const SESSION_FILE = "e2e/.auth/admin.json";

export default defineConfig({
  testDir: "./e2e",
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
    // The login flow itself has to run without a stored session.
    {
      name: "unauthenticated",
      testDir: "./e2e/unauthenticated",
    },
    // Everything else reuses the session the setup project stored, so no other
    // spec pays for a UI login or re-tests it incidentally.
    {
      name: "authenticated",
      testDir: "./e2e/authenticated",
      dependencies: ["setup"],
      use: { storageState: SESSION_FILE },
    },
  ],
  ...(!process.env.ELLA_E2E_BASE_URL && {
    webServer: {
      // Torn down first so each run starts from an empty database and a
      // container left behind by an interrupted run cannot block the port.
      command:
        "docker compose -f e2e/compose.yaml down -v && " +
        "docker compose -f e2e/compose.yaml up",
      env: { ELLA_E2E_PORT: E2E_PORT },
      url: `${DEFAULT_URL}/api/v1/status`,
      reuseExistingServer: false,
      timeout: 120_000,
      stdout: "pipe",
      stderr: "pipe",
    },
  }),
});
