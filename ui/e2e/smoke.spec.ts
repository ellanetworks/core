// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { test, expect } from "@playwright/test";
import {
  ADMIN_EMAIL,
  ADMIN_PASSWORD,
  adminToken,
  deleteSubscriberIfPresent,
} from "./api";

const PROFILE = "default";
const KEY = "00112233445566778899aabbccddeeff";
const SEQUENCE_NUMBER = "000000000001";

let token: string;
let msin: string;
let imsi: string;

test.beforeAll(async ({ request }) => {
  token = await adminToken(request);
});

test.beforeEach(async ({ request }) => {
  msin = String(Math.floor(Math.random() * 10_000_000_000)).padStart(10, "0");
  await deleteSubscriberIfPresent(request, token, msin);
});

test.afterEach(async ({ request }) => {
  if (imsi) await deleteSubscriberIfPresent(request, token, imsi);
});

test("an operator can sign in and manage a subscriber", async ({ page }) => {
  await test.step("the root path routes an initialized core to login", async () => {
    await page.goto("/");
    await expect(page).toHaveURL(/\/login$/);
  });

  await test.step("signing in lands on the dashboard", async () => {
    await page.getByLabel(/^Email/).fill(ADMIN_EMAIL);
    await page.getByLabel(/^Password/).fill(ADMIN_PASSWORD);
    await page.getByRole("button", { name: "Login" }).click();

    await expect(page).toHaveURL(/\/dashboard$/);
  });

  await test.step("the subscribers page is reachable from the nav", async () => {
    await page.getByRole("link", { name: "Subscribers" }).click();

    await expect(page).toHaveURL(/\/subscribers$/);
    await expect(page.getByRole("button", { name: "Create" })).toBeVisible();
  });

  await test.step("a new subscriber can be created", async () => {
    await page.getByRole("button", { name: "Create" }).click();

    const dialog = page.getByRole("dialog", { name: "Create Subscriber" });
    await expect(dialog).toBeVisible();

    await dialog.getByLabel(/^IMSI/).fill(msin);
    await dialog.getByLabel(/^Key/).fill(KEY);
    await dialog.getByLabel(/^Sequence Number/).fill(SEQUENCE_NUMBER);
    await dialog.getByRole("combobox").click();
    await page.getByRole("option", { name: PROFILE, exact: true }).click();

    await dialog.getByRole("button", { name: "Create" }).click();
    await expect(dialog).toBeHidden();
  });

  await test.step("the new subscriber appears in the list", async () => {
    const row = page.getByRole("row").filter({ hasText: msin });
    await expect(row).toBeVisible();

    imsi = (await row.getByRole("gridcell").first().innerText()).trim();
    expect(imsi).toContain(msin);
  });

  await test.step("its detail page opens", async () => {
    await page.getByRole("link", { name: imsi }).click();

    await expect(page).toHaveURL(new RegExp(`/subscribers/${imsi}$`));
    await expect(page.getByText(imsi).first()).toBeVisible();
  });

  await test.step("deleting it removes it from the list", async () => {
    await page.getByRole("button", { name: "Delete" }).click();

    const confirm = page.getByRole("dialog");
    await expect(confirm).toBeVisible();
    await confirm.getByRole("button", { name: "Confirm" }).click();

    await expect(page).toHaveURL(/\/subscribers$/);
    await expect(page.getByRole("row").filter({ hasText: msin })).toHaveCount(
      0,
    );
  });
});
