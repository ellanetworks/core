// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { test, expect, type Page } from "@playwright/test";
import { assertNoA11yViolations } from "../a11y";

const ERROR_BOUNDARY = "Something went wrong in the interface";

const ROUTES = [
  { label: "Dashboard", route: "/dashboard", heading: /^Ella Core/ },
  { label: "Subscribers", route: "/subscribers", heading: /^Subscribers/ },
  { label: "Profiles", route: "/profiles", heading: /^Profiles/ },
  { label: "Radios", route: "/radios", heading: /^Radios/ },
  {
    label: "Network events",
    route: "/radios/events",
    heading: "Network Events",
  },
  {
    label: "Data networks",
    route: "/networking/data-networks",
    heading: "Networking",
  },
  { label: "Slices", route: "/networking/slices", heading: "Networking" },
  {
    label: "Interfaces",
    route: "/networking/interfaces",
    heading: "Networking",
  },
  { label: "Routes", route: "/networking/routes", heading: "Networking" },
  { label: "NAT", route: "/networking/nat", heading: "Networking" },
  { label: "BGP", route: "/networking/bgp", heading: "Networking" },
  {
    label: "Flow accounting",
    route: "/networking/flow-accounting",
    heading: "Networking",
  },
  {
    label: "Local switch",
    route: "/networking/local-switch",
    heading: "Networking",
  },
  { label: "Operator", route: "/operator", heading: "Operator" },
  { label: "Traffic usage", route: "/traffic/usage", heading: "Traffic" },
  { label: "Traffic flows", route: "/traffic/flows", heading: "Traffic" },
  { label: "My profile", route: "/profile", heading: "My Profile" },
  { label: "Users", route: "/users", heading: /^Users/ },
  { label: "Audit logs", route: "/audit-logs", heading: "Audit Logs" },
  {
    label: "Backup and restore",
    route: "/backup-restore",
    heading: "Backup & Restore",
  },
  { label: "Cluster", route: "/cluster", heading: "Cluster" },
] as const;

const settle = async (page: Page, heading: string | RegExp, route: string) => {
  const title = page.getByRole("heading", { name: heading }).first();
  const boundary = page.getByText(ERROR_BOUNDARY);

  await expect(
    title.or(boundary).first(),
    `${route} rendered neither its heading nor an error`,
  ).toBeVisible();

  await expect(boundary, `${route} hit the error boundary`).toHaveCount(0);
  await expect(title, `${route} did not render its heading`).toBeVisible();

  await expect(page.getByRole("progressbar")).toHaveCount(0, {
    timeout: 15_000,
  });
};

for (const { label, route, heading } of ROUTES) {
  test(`${label} renders`, async ({ page }) => {
    const pageErrors: string[] = [];
    page.on("pageerror", (error) => pageErrors.push(String(error)));

    await page.goto(route);
    await settle(page, heading, route);

    const body = (await page.locator("body").innerText()).trim();
    for (const placeholder of [
      /\bundefined\b/,
      /\bNaN\b/,
      /\[object Object\]/,
    ]) {
      expect(body, `${route} rendered a placeholder value`).not.toMatch(
        placeholder,
      );
    }

    expect(pageErrors, `${route} raised an uncaught error`).toEqual([]);
  });

  test(`${label} has no accessibility violations`, async ({ page }) => {
    await page.goto(route);
    await settle(page, heading, route);

    await assertNoA11yViolations(page, label);
  });
}
