// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { rmSync } from "node:fs";

export default function globalSetup() {
  rmSync("e2e/.auth", { recursive: true, force: true });
}
