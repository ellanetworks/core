// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";

const apiTarget = process.env.ELLA_API_PROXY_TARGET ?? "http://localhost:5000";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "src"),
    },
  },
  server: {
    port: 3005,
    proxy: {
      "/api": {
        target: apiTarget,
        changeOrigin: true,
      },
      "/metrics": {
        target: apiTarget,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    chunkSizeWarningLimit: 700,
    rolldownOptions: {
      output: {
        manualChunks(id) {
          if (
            id.includes("react-router-dom") ||
            id.includes("node_modules/react/") ||
            id.includes("node_modules/react-dom/")
          ) {
            return "vendor";
          }
          if (
            id.includes("@mui/x-charts") ||
            id.includes("@mui/x-date-pickers")
          ) {
            return "mui-x-charts";
          }
          if (id.includes("@mui/x-data-grid")) {
            return "mui-x";
          }
          if (
            id.includes("@mui/material") ||
            id.includes("@mui/icons-material") ||
            id.includes("@emotion/")
          ) {
            return "mui";
          }
        },
      },
    },
  },
});
