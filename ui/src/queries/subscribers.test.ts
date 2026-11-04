// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi, afterEach } from "vitest";
import {
  SUBSCRIBER_SEARCH_LIMIT,
  listSubscribers,
  searchSubscriberImsis,
} from "./subscribers";

const stubSearch = (imsis: string[]) => {
  const requested: string[] = [];

  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) => {
      requested.push(url);

      return {
        status: 200,
        ok: true,
        statusText: "OK",
        json: async () => ({
          result: {
            items: imsis.map((imsi) => ({ imsi })),
            total_count: 100_000,
          },
        }),
      } as Response;
    }),
  );

  return requested;
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("searchSubscriberImsis", () => {
  it("fetches a single bounded page, whatever the total", async () => {
    const requested = stubSearch(["001", "002"]);

    await expect(searchSubscriberImsis("t0k", "")).resolves.toEqual([
      "001",
      "002",
    ]);

    expect(requested).toHaveLength(1);
    expect(new URL(requested[0], "http://x").search).toBe(
      `?page=1&per_page=${SUBSCRIBER_SEARCH_LIMIT}`,
    );
  });

  it("sends a trimmed search term and the data network", async () => {
    const requested = stubSearch([]);

    await searchSubscriberImsis("t0k", " 0748 ", "internet");

    const params = new URL(requested[0], "http://x").searchParams;
    expect(params.get("search")).toBe("0748");
    expect(params.get("data_network")).toBe("internet");
  });

  it("omits a blank search term", async () => {
    const requested = stubSearch([]);

    await searchSubscriberImsis("t0k", "   ");

    expect(new URL(requested[0], "http://x").searchParams.has("search")).toBe(
      false,
    );
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

    await expect(searchSubscriberImsis("t0k", "")).resolves.toEqual([]);
  });

  it("propagates a failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => ({
        status: 403,
        ok: false,
        statusText: "Forbidden",
        json: async () => ({ error: "denied" }),
      })),
    );

    await expect(searchSubscriberImsis("t0k", "")).rejects.toMatchObject({
      status: 403,
    });
  });

  it("sends the token", async () => {
    stubSearch([]);

    await searchSubscriberImsis("t0k", "");

    expect(fetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: "Bearer t0k" }),
      }),
    );
  });
});

describe("listSubscribers", () => {
  const stubOk = () => {
    const requested: string[] = [];

    vi.stubGlobal(
      "fetch",
      vi.fn(async (url: string) => {
        requested.push(url);
        return {
          status: 200,
          ok: true,
          statusText: "OK",
          json: async () => ({ result: { items: [], total_count: 0 } }),
        } as Response;
      }),
    );

    return () => new URL(requested[0], "http://x");
  };

  it("omits search when it is not given", async () => {
    const requested = stubOk();

    await listSubscribers("t0k", 2, 25);

    expect(requested().search).toBe("?page=2&per_page=25");
  });

  it("omits search when it is empty", async () => {
    const requested = stubOk();

    await listSubscribers("t0k", 1, 25, "");

    expect(requested().searchParams.has("search")).toBe(false);
  });

  it("sends search alongside the pagination params", async () => {
    const requested = stubOk();

    await listSubscribers("t0k", 1, 25, "0748");

    expect(requested().searchParams.get("search")).toBe("0748");
  });

  it("encodes a search that would otherwise break the query string", async () => {
    const requested = stubOk();

    await listSubscribers("t0k", 1, 25, "a b&per_page=1");

    const url = requested();
    expect(url.searchParams.get("search")).toBe("a b&per_page=1");
    expect(url.searchParams.get("per_page")).toBe("25");
  });
});
