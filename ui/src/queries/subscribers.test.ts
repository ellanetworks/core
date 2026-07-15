// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi, afterEach } from "vitest";
import { listAllSubscriberImsis } from "./subscribers";

const PER_PAGE = 100;

/**
 * Serves `total` IMSIs across pages, recording the requested page numbers so a
 * test can assert how the roster was assembled rather than only what came back.
 */
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
        (_v, i) => ({ imsi: String(900_000 + start + i) }),
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

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("listAllSubscriberImsis", () => {
  // The API caps per_page at 100 while MaxNumSubscribers is 1000, so the page
  // count is the load-bearing arithmetic.
  it.each([
    [0, 1],
    [1, 1],
    [99, 1],
    [100, 1],
    [101, 2],
    [200, 2],
    [201, 3],
    [1000, 10],
  ])("fetches %i subscribers in %i request(s)", async (total, requests) => {
    const pages = stubRoster(total);

    const imsis = await listAllSubscriberImsis("t0k");

    expect(imsis).toHaveLength(total);
    expect(pages).toHaveLength(requests);
  });

  it("requests each page exactly once, starting at 1", async () => {
    const pages = stubRoster(250);

    await listAllSubscriberImsis("t0k");

    expect([...pages].sort((a, b) => a - b)).toEqual([1, 2, 3]);
  });

  it("returns every IMSI across the page boundary, without duplicates", async () => {
    stubRoster(150);

    const imsis = await listAllSubscriberImsis("t0k");

    expect(new Set(imsis).size).toBe(150);
    expect(imsis).toContain("900099"); // last of page 1
    expect(imsis).toContain("900100"); // first of page 2
  });

  it("sorts, so the filter's order does not depend on usage or page arrival", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        status: 200,
        ok: true,
        statusText: "OK",
        json: async () => ({
          result: {
            items: [{ imsi: "003" }, { imsi: "001" }, { imsi: "002" }],
            total_count: 3,
          },
        }),
      })),
    );

    await expect(listAllSubscriberImsis("t0k")).resolves.toEqual([
      "001",
      "002",
      "003",
    ]);
  });

  it("falls back to the item count when total_count is absent", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        status: 200,
        ok: true,
        statusText: "OK",
        json: async () => ({ result: { items: [{ imsi: "001" }] } }),
      })),
    );

    await expect(listAllSubscriberImsis("t0k")).resolves.toEqual(["001"]);
  });

  it("tolerates a page with no items array", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        status: 200,
        ok: true,
        statusText: "OK",
        json: async () => ({ result: {} }),
      })),
    );

    await expect(listAllSubscriberImsis("t0k")).resolves.toEqual([]);
  });

  it("propagates a failure rather than returning a short roster", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        status: 403,
        ok: false,
        statusText: "Forbidden",
        json: async () => ({ error: "denied" }),
      })),
    );

    await expect(listAllSubscriberImsis("t0k")).rejects.toMatchObject({
      status: 403,
    });
  });

  it("sends the token and the per_page cap", async () => {
    stubRoster(1);

    await listAllSubscriberImsis("t0k");

    expect(fetch).toHaveBeenCalledWith(
      `/api/v1/subscribers?page=1&per_page=${PER_PAGE}`,
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: "Bearer t0k" }),
      }),
    );
  });
});
