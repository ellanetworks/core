// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { execFileSync } from "node:child_process";

// Playwright terminates the `docker compose up` process between runs but leaves
// the containers it started behind, which then holds the port and fails the next
// run's server check. Bringing the project down here also drops the volume, so
// every run starts from an empty database.
export default function globalTeardown() {
  if (process.env.ELLA_E2E_BASE_URL) return;

  try {
    execFileSync(
      "docker",
      ["compose", "-f", "e2e/compose.yaml", "down", "-v"],
      { stdio: "ignore" },
    );
  } catch {
    // A run that never started the stack has nothing to tear down.
  }
}
