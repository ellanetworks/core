// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  ApiError,
  HTTPStatus,
  apiFetch,
  apiFetchVoid,
  setOnUnauthorized,
} from "./utils";

const jsonResponse = (status: number, body: unknown): Response =>
  ({
    status,
    ok: status >= 200 && status < 300,
    statusText: HTTPStatus(status),
    json: async () => body,
  }) as Response;

const unparseableResponse = (status: number, statusText = ""): Response =>
  ({
    status,
    ok: status >= 200 && status < 300,
    statusText,
    json: async (): Promise<unknown> => {
      throw new SyntaxError("Unexpected token");
    },
  }) as Response;

const stubFetch = (impl: () => Promise<Response>) => {
  vi.stubGlobal("fetch", vi.fn(impl));
};

beforeEach(() => {
  setOnUnauthorized(null);
});

afterEach(() => {
  vi.unstubAllGlobals();
  setOnUnauthorized(null);
});

describe("ApiError.retryable", () => {
  // Retry is offered for failures that may pass on a second attempt; a 400 is
  // the request's own fault and never will.
  it.each([
    ["network", undefined, true],
    ["http", 500, true],
    ["http", 502, true],
    ["http", 503, true],
    ["http", 400, false],
    ["http", 404, false],
    ["http", 409, false],
    ["auth", 401, false],
    ["forbidden", 403, false],
  ] as const)("%s / %s -> retryable=%s", (kind, status, expected) => {
    expect(new ApiError(kind, "x", { status }).retryable).toBe(expected);
  });

  it("is not retryable when an http error carries no status", () => {
    expect(new ApiError("http", "x").retryable).toBe(false);
  });
});

describe("ApiError", () => {
  it("is an Error, so instanceof and .message survive the class extension", () => {
    const err = new ApiError("http", "boom", { status: 400, detail: "d" });

    expect(err).toBeInstanceOf(Error);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toBe("boom");
    expect(err.name).toBe("ApiError");
    expect(err.detail).toBe("d");
  });
});

describe("HTTPStatus", () => {
  it.each([
    [400, "Bad Request"],
    [401, "Unauthorized"],
    [403, "Forbidden"],
    [409, "Conflict"],
    [500, "Internal Server Error"],
  ])("%i -> %s", (code, text) => {
    expect(HTTPStatus(code)).toBe(text);
  });

  it("falls back for an unmapped code", () => {
    expect(HTTPStatus(418)).toBe("HTTP Error 418");
  });
});

describe("apiFetch error mapping", () => {
  // The backend's own message is what explains a rejection — e.g. the RAT-aware
  // bitrate rules. Losing it would degrade every 400 to "Bad Request".
  it("surfaces the backend's message verbatim", async () => {
    stubFetch(async () =>
      jsonResponse(400, {
        error:
          "session_ambr_uplink of 20 Gbps exceeds the 10 Gbps ceiling of a profile that allows 4G (TS 24.008 §10.5.6.5B)",
      }),
    );

    await expect(apiFetch("/api/v1/policies")).rejects.toThrow(
      /exceeds the 10 Gbps ceiling/,
    );
  });

  it.each([
    [401, "auth"],
    [403, "forbidden"],
    [400, "http"],
    [409, "http"],
    [500, "http"],
  ] as const)("maps %i to kind %s", async (status, kind) => {
    stubFetch(async () => jsonResponse(status, { error: "backend text" }));

    await expect(apiFetch("/x")).rejects.toMatchObject({
      kind,
      status,
      message: "backend text",
    });
  });

  it.each([
    [401, "Your session has expired. Please log in again."],
    [403, "You do not have permission to perform this action."],
    [409, "Conflict"],
  ])(
    "falls back to its own text for %i when the body has none",
    async (status, expected) => {
      stubFetch(async () => jsonResponse(status, {}));

      await expect(apiFetch("/x")).rejects.toMatchObject({ message: expected });
    },
  );

  it("reports an unreachable server as network, not http", async () => {
    stubFetch(async () => {
      throw new TypeError("Failed to fetch");
    });

    await expect(apiFetch("/x")).rejects.toMatchObject({
      kind: "network",
      retryable: true,
    });
  });

  it("carries a detail line alongside the primary message", async () => {
    stubFetch(async () => jsonResponse(500, { error: "boom" }));

    await expect(apiFetch("/x")).rejects.toMatchObject({
      message: "boom",
      detail: "HTTP 500 Internal Server Error",
    });
  });

  it("uses statusText when the error body will not parse", async () => {
    stubFetch(async () => unparseableResponse(502, "Bad Gateway"));

    await expect(apiFetch("/x")).rejects.toMatchObject({
      kind: "http",
      status: 502,
      message: "Bad Gateway",
    });
  });
});

describe("apiFetch success", () => {
  it("unwraps result", async () => {
    stubFetch(async () => jsonResponse(200, { result: { imsi: "001" } }));

    await expect(apiFetch("/x")).resolves.toEqual({ imsi: "001" });
  });

  it("treats an ok response with an unparseable body as an empty success", async () => {
    stubFetch(async () => unparseableResponse(200));

    await expect(apiFetch("/x")).resolves.toBeUndefined();
  });

  it("sends the bearer token and serialises the body", async () => {
    stubFetch(async () => jsonResponse(200, { result: null }));

    await apiFetch("/x", { method: "POST", authToken: "t0k", body: { a: 1 } });

    expect(fetch).toHaveBeenCalledWith("/x", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer t0k",
      },
      body: '{"a":1}',
    });
  });

  it("omits the Authorization header when there is no token", async () => {
    stubFetch(async () => jsonResponse(200, { result: null }));

    await apiFetch("/x");

    expect(fetch).toHaveBeenCalledWith("/x", {
      method: "GET",
      headers: { "Content-Type": "application/json" },
    });
  });
});

describe("onUnauthorized", () => {
  it("fires on 401", async () => {
    const cb = vi.fn();
    setOnUnauthorized(cb);
    stubFetch(async () => jsonResponse(401, { error: "nope" }));

    await expect(apiFetch("/x")).rejects.toThrow();
    expect(cb).toHaveBeenCalledOnce();
  });

  it.each([403, 500, 200])("does not fire on %i", async (status) => {
    const cb = vi.fn();
    setOnUnauthorized(cb);
    stubFetch(async () => jsonResponse(status, { result: null, error: "e" }));

    await apiFetch("/x").catch(() => undefined);

    expect(cb).not.toHaveBeenCalled();
  });
});

describe("apiFetchVoid", () => {
  it("resolves on success without a body", async () => {
    stubFetch(async () => jsonResponse(204, {}));

    await expect(
      apiFetchVoid("/x", { method: "DELETE" }),
    ).resolves.toBeUndefined();
  });

  it("surfaces the backend's message on failure", async () => {
    stubFetch(async () => jsonResponse(409, { error: "already exists" }));

    await expect(apiFetchVoid("/x", { method: "POST" })).rejects.toMatchObject({
      kind: "http",
      status: 409,
      message: "already exists",
    });
  });
});
