// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi, afterEach } from "vitest";
import {
  deregisterRoamer,
  listRoamerPartners,
  listRoamers,
  listRoamersByDataNetwork,
  listRoamersByRadio,
} from "./roamers";

const PER_PAGE = 100;

const stubRoster = (total: number) => {
  const pagesRequested: number[] = [];

  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      const page = Number(new URL(url, "http://x").searchParams.get("page"));
      pagesRequested.push(page);

      const start = (page - 1) * PER_PAGE;
      const items = Array.from(
        { length: Math.max(0, Math.min(PER_PAGE, total - start)) },
        (_v, i) => ({
          imsi: String(900_000 + start + i),
          partner: "Partner A",
        }),
      );

      return {
        status: 200,
        ok: true,
        statusText: "OK",
        json: async () => ({ result: { items, total_count: total } }),
      } as Response;
    }),
  );

  return pagesRequested;
};

const requestedURL = (call = 0) =>
  new URL(vi.mocked(fetch).mock.calls[call][0] as string, "http://x");

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("listRoamers", () => {
  it("sends the search and partner only when set", async () => {
    stubRoster(0);

    await listRoamers("t0k", 2, 25);
    await listRoamers("t0k", 1, 25, "2089", "Partner A");

    const plain = requestedURL(0);
    expect(plain.pathname).toBe("/api/v1/roamers");
    expect(plain.searchParams.get("page")).toBe("2");
    expect(plain.searchParams.has("search")).toBe(false);
    expect(plain.searchParams.has("partner")).toBe(false);

    const filtered = requestedURL(1);
    expect(filtered.searchParams.get("search")).toBe("2089");
    expect(filtered.searchParams.get("partner")).toBe("Partner A");
  });
});

describe("listRoamersByRadio", () => {
  it("filters by radio", async () => {
    stubRoster(0);

    await listRoamersByRadio("t0k", "gnb 01", 2, 10);

    const url = requestedURL();
    expect(url.pathname).toBe("/api/v1/roamers");
    expect(url.searchParams.get("radio")).toBe("gnb 01");
    expect(url.searchParams.get("page")).toBe("2");
    expect(url.searchParams.get("per_page")).toBe("10");
  });
});

describe("listRoamersByDataNetwork", () => {
  it("filters by data network", async () => {
    stubRoster(0);

    await listRoamersByDataNetwork("t0k", "internet", 1, 10);

    const url = requestedURL();
    expect(url.pathname).toBe("/api/v1/roamers");
    expect(url.searchParams.get("data_network")).toBe("internet");
    expect(url.searchParams.has("radio")).toBe(false);
  });
});

describe("listRoamerPartners", () => {
  it("maps every roamer to its partner", async () => {
    const pages = stubRoster(150);

    const partners = await listRoamerPartners("t0k");

    expect(Object.keys(partners)).toHaveLength(150);
    expect(partners["900000"]).toBe("Partner A");
    expect(pages).toEqual([1, 2]);
    expect(requestedURL().pathname).toBe("/api/v1/roamers");
  });

  it("stops at 1000 roamers", async () => {
    const pages = stubRoster(1500);

    const partners = await listRoamerPartners("t0k");

    expect(Object.keys(partners)).toHaveLength(1000);
    expect(pages).toHaveLength(10);
  });
});

describe("deregisterRoamer", () => {
  it("deletes the roamer", async () => {
    stubRoster(0);

    await deregisterRoamer("t0k", "208930000000001");

    expect(requestedURL().pathname).toBe("/api/v1/roamers/208930000000001");
    expect(vi.mocked(fetch).mock.calls[0][1]?.method).toBe("DELETE");
  });
});
