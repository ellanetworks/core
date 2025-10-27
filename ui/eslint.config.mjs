import { defineConfig } from "eslint/config";
import next from "eslint-config-next";

export default defineConfig([
  ...next,
  {
    rules: {
      "@typescript-eslint/no-explicit-any": "off",
    },
  },
]);
