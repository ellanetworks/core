// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { test, expect } from "@playwright/test";
import { ensureInitialized } from "../api";
import { assertNoA11yViolations } from "../a11y";

test.beforeAll(async ({ request }) => {
  await ensureInitialized(request);
});

test.describe("dark mode", () => {
  test.use({ colorScheme: "dark" });

  test("the login page is accessible in dark mode", async ({ page }) => {
    await page.goto("/login");
    await expect(page.getByRole("button", { name: "Login" })).toBeVisible();
    await expect(page.locator("html")).toHaveClass(/dark/);

    await assertNoA11yViolations(page, "Login (dark)");
  });
});
