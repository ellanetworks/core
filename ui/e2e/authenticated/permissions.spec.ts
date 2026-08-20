// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { test, expect, type Browser, type Page } from "@playwright/test";
import { ROLES } from "../roles";

const ADMIN_ONLY_ROUTES = [
  "/users",
  "/audit-logs",
  "/backup-restore",
  "/cluster",
];

const asRole = async (browser: Browser, session: string): Promise<Page> => {
  const context = await browser.newContext({ storageState: session });
  return context.newPage();
};

for (const [label, role] of Object.entries(ROLES)) {
  test.describe(`as ${label}`, () => {
    for (const route of ADMIN_ONLY_ROUTES) {
      test(`${route} redirects away from the admin-only page`, async ({
        browser,
      }) => {
        const page = await asRole(browser, role.session);

        await page.goto(route);

        await expect(page).toHaveURL(/\/dashboard$/);
        await page.context().close();
      });
    }

    test("the System navigation section is hidden", async ({ browser }) => {
      const page = await asRole(browser, role.session);

      await page.goto("/dashboard");
      const nav = page.getByRole("navigation", { name: "Main" });
      await expect(
        nav.getByRole("link", { name: "Subscribers", exact: true }),
      ).toBeVisible();

      for (const item of [
        "Users",
        "Audit Logs",
        "Backup and Restore",
        "Cluster",
      ]) {
        await expect(
          nav.getByRole("link", { name: item, exact: true }),
        ).toHaveCount(0);
      }
      await page.context().close();
    });

    test(`the subscriber create action is ${role.canEdit ? "offered" : "withheld"}`, async ({
      browser,
    }) => {
      const page = await asRole(browser, role.session);

      await page.goto("/subscribers");
      await expect(
        page.getByRole("heading", { name: "Subscribers", level: 1 }),
      ).toBeVisible();

      const create = page.getByRole("button", { name: "Create" });
      if (role.canEdit) {
        await expect(create).toBeVisible();
      } else {
        await expect(create).toHaveCount(0);
      }
      await page.context().close();
    });
  });
}
