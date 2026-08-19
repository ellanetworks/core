// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { test, expect, type APIRequestContext } from "@playwright/test";
import {
  adminToken,
  deleteSubscriberIfPresent,
  imsiPrefix,
  listItems,
} from "../api";

const KEY = "00112233445566778899aabbccddeeff";
const SEQUENCE_NUMBER = "000000000001";

let token: string;
let seededImsi = "";

const RESOURCES = [
  {
    label: "Subscribers",
    route: "/subscribers",
    endpoint: "/api/v1/subscribers",
    required: ["imsi", "profile_name", "status"],
  },
  {
    label: "Data networks",
    route: "/networking/data-networks",
    endpoint: "/api/v1/networking/data-networks",
    required: ["name", "ipv4_pool", "dns", "mtu"],
  },
  {
    label: "Profiles",
    route: "/profiles",
    endpoint: "/api/v1/profiles",
    required: [
      "name",
      "ue_ambr_uplink",
      "ue_ambr_downlink",
      "allow_4g",
      "allow_5g",
    ],
  },
  {
    label: "Policies",
    route: "/profiles/default",
    endpoint: "/api/v1/policies",
    required: [
      "name",
      "profile_name",
      "slice_name",
      "data_network_name",
      "session_ambr_uplink",
      "session_ambr_downlink",
      "var5qi",
      "arp",
      "default",
    ],
  },
] as const;

const seedSubscriber = async (request: APIRequestContext) => {
  const msin = String(Math.floor(Math.random() * 10_000_000_000)).padStart(
    10,
    "0",
  );
  const imsi = `${await imsiPrefix(request, token)}${msin}`;

  const response = await request.post("/api/v1/subscribers", {
    headers: { Authorization: `Bearer ${token}` },
    data: {
      imsi,
      key: KEY,
      sequenceNumber: SEQUENCE_NUMBER,
      profile_name: "default",
      opc: "",
    },
  });

  const items = await listItems(request, token, "/api/v1/subscribers");
  const seeded = items.find((item) => item.imsi === imsi);

  if (!seeded) {
    throw new Error(
      `could not seed a subscriber: ${response.status()} ${await response.text()}`,
    );
  }

  return imsi;
};

// Seeded per test rather than per file: worker-scoped hooks run once in each
// worker, so a shared fixture gets torn down by one worker while another is
// still reading it.
test.beforeEach(async ({ request }) => {
  token = await adminToken(request);
  seededImsi = await seedSubscriber(request);
});

test.afterEach(async ({ request }) => {
  if (seededImsi) await deleteSubscriberIfPresent(request, token, seededImsi);
});

for (const resource of RESOURCES) {
  test(`${resource.label} responses carry every field the UI types as required`, async ({
    request,
  }) => {
    const items = await listItems(request, token, resource.endpoint);
    expect(
      items.length,
      `${resource.endpoint} returned no rows to check`,
    ).toBeGreaterThan(0);

    for (const item of items) {
      for (const field of resource.required) {
        expect(
          item[field],
          `${resource.endpoint} omitted "${field}" from ${JSON.stringify(item)}`,
        ).not.toBeUndefined();
        expect(
          item[field],
          `${resource.endpoint} sent "${field}" as null`,
        ).not.toBeNull();
      }
    }
  });

  test(`${resource.label} renders its rows without placeholder values`, async ({
    page,
  }) => {
    await page.goto(resource.route);

    const grid = page.getByRole("grid").first();
    await expect(grid).toBeVisible();
    await expect(grid.getByRole("row")).not.toHaveCount(0);

    const text = (await grid.innerText()).trim();
    expect(
      text.length,
      `${resource.route} rendered an empty grid`,
    ).toBeGreaterThan(0);
    expect(text).not.toMatch(/\bundefined\b/);
    expect(text).not.toMatch(/\bNaN\b/);
    expect(text).not.toMatch(/\[object Object\]/);
  });
}
