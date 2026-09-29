// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { defineConfig } from "vitest/config";
import path from "node:path";
import { DEFAULT_BRANDING_DIR, loadBranding } from "./branding/branding";

export default defineConfig({
  define: {
    BRANDING: JSON.stringify(loadBranding(DEFAULT_BRANDING_DIR).client),
  },
  resolve: {
    alias: { "@": path.resolve(import.meta.dirname, "./src") },
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.ts", "src/**/*.test.tsx", "branding/**/*.test.ts"],
    setupFiles: ["./vitest.setup.ts"],
    maxWorkers: 4,
    testTimeout: 15000,
  },
});
