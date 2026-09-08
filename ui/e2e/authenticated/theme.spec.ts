// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { test, expect, type Page } from "@playwright/test";
import { assertNoA11yViolations } from "../a11y";
import { adminToken, seedProtocolRules } from "../api";

const COLOUR_HEAVY_ROUTES = [
  { label: "Dashboard", route: "/dashboard" },
  { label: "Traffic flows", route: "/traffic/flows" },
  { label: "Subscribers", route: "/subscribers" },
  { label: "Audit logs", route: "/audit-logs" },
];

const htmlClass = (page: Page) => page.locator("html").getAttribute("class");

const collectCspViolations = async (page: Page) => {
  const violations: string[] = [];
  await page.addInitScript(() => {
    (window as Window & { cspViolations?: string[] }).cspViolations = [];
    document.addEventListener("securitypolicyviolation", (event) => {
      (window as Window & { cspViolations?: string[] }).cspViolations?.push(
        `${event.violatedDirective} ${event.blockedURI}`,
      );
    });
  });
  return async () => {
    violations.push(
      ...(await page.evaluate(
        () =>
          (window as Window & { cspViolations?: string[] }).cspViolations ?? [],
      )),
    );
    return violations;
  };
};

const schemeBeforeBundle = async (page: Page, route: string) => {
  await page.addInitScript(() => {
    document.addEventListener("readystatechange", () => {
      const w = window as Window & { earlyColorScheme?: string };
      if (document.readyState === "interactive" && !w.earlyColorScheme) {
        w.earlyColorScheme = getComputedStyle(
          document.documentElement,
        ).colorScheme;
      }
    });
  });
  await page.goto(route, { waitUntil: "commit" });
  await page.waitForLoadState("load");
  return page.evaluate(
    () =>
      (window as Window & { earlyColorScheme?: string }).earlyColorScheme ??
      "(not sampled)",
  );
};

const chooseMode = async (page: Page, name: string) => {
  await page.getByRole("button", { name: "account menu" }).click();
  await page.getByRole("menuitemradio", { name: `${name} theme` }).click();
  await expect(page.getByRole("menu")).toBeHidden();
};

test.describe("dark mode", () => {
  test.use({ colorScheme: "dark" });

  test("paints the OS scheme before the bundle runs", async ({ page }) => {
    expect(await schemeBeforeBundle(page, "/dashboard")).toBe("dark");
  });

  test("loads without tripping the app's own CSP", async ({ page }) => {
    const read = await collectCspViolations(page);
    await page.goto("/dashboard");
    await expect(page.getByRole("progressbar")).toHaveCount(0, {
      timeout: 15_000,
    });

    expect(await read(), "the page must not violate its own CSP").toEqual([]);
  });

  test("applies the scheme class once mounted", async ({ page }) => {
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

  test("groups the theme options under a labelled group", async ({ page }) => {
    await page.goto("/dashboard");
    await page.getByRole("button", { name: "account menu" }).click();

    const group = page.getByRole("group", { name: "Theme" });
    await expect(group).toBeVisible();
    await expect(group.getByRole("menuitemradio")).toHaveCount(3);
  });

  test("the policy detail protocol chips are accessible", async ({
    page,
    request,
  }) => {
    await seedProtocolRules(request, await adminToken(request));

    await page.goto("/profiles/default/policies/default");
    await expect(page.getByRole("progressbar")).toHaveCount(0, {
      timeout: 15_000,
    });
    await expect(page.getByText("UDP").first()).toBeVisible();

    await assertNoA11yViolations(page, "Policy detail chips (dark)");
  });

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

  test("paints the OS scheme before the bundle runs", async ({ page }) => {
    expect(await schemeBeforeBundle(page, "/dashboard")).toBe("light");
  });

  test("applies the scheme class once mounted", async ({ page }) => {
    await page.goto("/dashboard");

    expect(await htmlClass(page)).toContain("light");
  });
});
