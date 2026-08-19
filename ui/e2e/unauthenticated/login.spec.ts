// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { test, expect } from "@playwright/test";
import { ADMIN_EMAIL, ADMIN_PASSWORD, ensureInitialized } from "../api";

test.beforeAll(async ({ request }) => {
  await ensureInitialized(request);
});

test("an initialized core routes the root path to login", async ({ page }) => {
  await page.goto("/");

  await expect(page).toHaveURL(/\/login$/);
});

test("signing in lands on the dashboard", async ({ page }) => {
  await page.goto("/login");

  await page.getByLabel(/^Email/).fill(ADMIN_EMAIL);
  await page.getByLabel(/^Password/).fill(ADMIN_PASSWORD);
  await page.getByRole("button", { name: "Login" }).click();

  await expect(page).toHaveURL(/\/dashboard$/);
});

test("bad credentials keep the operator on the login page", async ({
  page,
}) => {
  await page.goto("/login");

  await page.getByLabel(/^Email/).fill(ADMIN_EMAIL);
  await page.getByLabel(/^Password/).fill("wrong-password");
  await page.getByRole("button", { name: "Login" }).click();

  await expect(page.getByRole("alert")).toBeVisible();
  await expect(page).toHaveURL(/\/login$/);
});
