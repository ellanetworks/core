// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { test, expect, type Page } from "@playwright/test";
import { assertNoA11yViolations } from "../a11y";

const COLOUR_HEAVY_ROUTES = [
  { label: "Dashboard", route: "/dashboard" },
  { label: "Traffic flows", route: "/traffic/flows" },
  { label: "Subscribers", route: "/subscribers" },
  { label: "Audit logs", route: "/audit-logs" },
];

const htmlClass = (page: Page) => page.locator("html").getAttribute("class");

const chooseMode = async (page: Page, name: string) => {
  await page.getByRole("button", { name: "account menu" }).click();
  await page.getByRole("menuitemradio", { name: `${name} theme` }).click();
  await page.keyboard.press("Escape");
};

test.describe("dark mode", () => {
  test.use({ colorScheme: "dark" });

  test("follows the OS preference before the app boots", async ({ page }) => {
    await page.goto("/dashboard");

    expect(await htmlClass(page)).toContain("dark");
  });

  for (const { label, route } of COLOUR_HEAVY_ROUTES) {
    test(`${label} is accessible in dark mode`, async ({ page }) => {
      await page.goto(route);
      await expect(page.getByRole("progressbar")).toHaveCount(0, {
        timeout: 15_000,
      });

      await assertNoA11yViolations(page, `${label} (dark)`);
    });
  }

  test("the account menu is accessible with the theme options open", async ({
    page,
  }) => {
    await page.goto("/dashboard");
    await page.getByRole("button", { name: "account menu" }).click();
    await expect(
      page.getByRole("menuitemradio", { name: "Dark theme" }),
    ).toBeVisible();

    await assertNoA11yViolations(page, "account menu (dark)");
  });

  test("an explicit choice overrides the OS and survives a reload", async ({
    page,
  }) => {
    await page.goto("/dashboard");
    await chooseMode(page, "Light");

    await expect(page.locator("html")).toHaveClass(/light/);

    await page.reload();

    await expect(page.locator("html")).toHaveClass(/light/);
    await expect(page.locator("html")).not.toHaveClass(/dark/);
  });
});

test.describe("light mode", () => {
  test.use({ colorScheme: "light" });

  test("follows the OS preference before the app boots", async ({ page }) => {
    await page.goto("/dashboard");

    expect(await htmlClass(page)).toContain("light");
  });
});
