// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { defineConfig } from "eslint/config";

export default defineConfig([
  {
    rules: {
      "@typescript-eslint/no-explicit-any": "off",
    },
  },
]);
