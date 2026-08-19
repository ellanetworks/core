// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { execFileSync } from "node:child_process";

export default function globalTeardown() {
  if (process.env.ELLA_E2E_BASE_URL) return;

  try {
    execFileSync(
      "docker",
      ["compose", "-f", "e2e/compose.yaml", "down", "-v"],
      { stdio: "ignore" },
    );
  } catch {}
}
