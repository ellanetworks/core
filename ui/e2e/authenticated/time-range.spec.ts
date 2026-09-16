// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { test, expect, type Page } from "@playwright/test";
import { assertNoA11yViolations } from "../a11y";

test.use({ timezoneId: "America/Toronto" });

const trigger = (page: Page) =>
  page.getByRole("button", { name: /^Time range:/ });

const openPicker = async (page: Page) => {
  await trigger(page).click();
  await expect(page.getByLabel("From", { exact: true })).toBeVisible();
};

const eventParams = (url: string) => new URL(url).searchParams;

test("Network Events resolves a relative range into a sliding lower bound", async ({
  page,
}) => {
  await page.goto("/radios/events");
  await expect(
    page.getByRole("heading", { name: "Network Events" }),
  ).toBeVisible();

  const request = page.waitForRequest(
    (req) =>
      req.url().includes("/api/v1/ran/events") &&
      eventParams(req.url()).has("start"),
  );

  await openPicker(page);
  await page.getByRole("menuitem", { name: "Last 15 minutes" }).click();

  const params = eventParams((await request).url());
  const elapsed = Date.now() - Date.parse(params.get("start")!);
  expect(elapsed).toBeGreaterThan(14 * 60_000);
  expect(elapsed).toBeLessThan(16 * 60_000);
  expect(params.has("relative_range")).toBe(false);
  await expect(trigger(page)).toHaveAccessibleName(
    "Time range: Last 15 minutes",
  );
});

test("Network Events refuses to query an inverted custom range", async ({
  page,
}) => {
  await page.goto("/radios/events");
  await expect(
    page.getByRole("heading", { name: "Network Events" }),
  ).toBeVisible();
  await openPicker(page);

  await page.getByLabel("From", { exact: true }).fill("2026-08-10T10:00");
  await page.getByLabel("To", { exact: true }).fill("2026-08-01T10:00");

  await expect(page.getByRole("alert")).toContainText(/on or after/i);

  const inverted: string[] = [];
  page.on("request", (req) => {
    if (!req.url().includes("/api/v1/ran/events")) return;
    const params = eventParams(req.url());
    const from = params.get("start");
    const to = params.get("end");
    if (from && to && from > to) inverted.push(req.url());
  });
  await page.waitForTimeout(1_000);
  expect(inverted).toEqual([]);
});

test("Audit Logs keeps a custom range on the instants it was given", async ({
  page,
}) => {
  await page.goto(
    "/audit-logs?range=custom&start=2026-09-04T09:00:00-04:00&end=2026-10-23T17:30:00-04:00",
  );
  await expect(page.getByRole("heading", { name: "Audit Logs" })).toBeVisible();

  await expect(trigger(page)).toHaveAccessibleName(
    /^Time range: Sep 4,.*09:00 → Oct 23,.*17:30$/,
  );

  await openPicker(page);
  await expect(page.getByLabel("From", { exact: true })).toHaveValue(
    "2026-09-04T09:00",
  );
  await expect(page.getByLabel("To", { exact: true })).toHaveValue(
    "2026-10-23T17:30",
  );
});

test("Audit Logs sends the instants a relative range resolves to", async ({
  page,
}) => {
  await page.goto("/audit-logs");
  await expect(page.getByRole("heading", { name: "Audit Logs" })).toBeVisible();

  const request = page.waitForRequest((req) =>
    req.url().includes("/api/v1/logs/audit?"),
  );

  await openPicker(page);
  await page.getByRole("menuitem", { name: "Last 15 minutes" }).click();

  const params = eventParams((await request).url());
  await expect
    .poll(async () => eventParams(page.url()).get("range"))
    .toBe("15m");
  const elapsed = Date.now() - Date.parse(params.get("start")!);
  expect(elapsed).toBeGreaterThan(14 * 60_000);
  expect(elapsed).toBeLessThan(16 * 60_000);
});

test("Traffic prefills the custom bounds from the active preset", async ({
  page,
}) => {
  await page.goto("/traffic/usage");
  await expect(page.getByRole("heading", { name: "Traffic" })).toBeVisible();
  await expect(trigger(page)).toHaveAccessibleName("Time range: Last 7 days");

  await openPicker(page);

  const from = await page.getByLabel("From", { exact: true }).inputValue();
  const to = await page.getByLabel("To", { exact: true }).inputValue();
  expect(from).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  expect(to).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  const spanDays =
    Math.round((Date.parse(to) - Date.parse(from)) / 86_400_000) + 1;
  expect(spanDays).toBe(7);

  await expect(page.getByRole("menuitem", { name: "Any time" })).toHaveCount(0);
});

test("the time range panel has no accessibility violations", async ({
  page,
}) => {
  await page.goto("/radios/events");
  await expect(
    page.getByRole("heading", { name: "Network Events" }),
  ).toBeVisible();
  await openPicker(page);

  await assertNoA11yViolations(page, "network events time range panel");
});
