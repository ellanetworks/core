// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { ApiServer, httpError, rawBody } from "./apiServer";

const server = () => new ApiServer();

describe("apiServer request capture", () => {
  it("records the Authorization header", async () => {
    const api = server();
    api.get("/api/v1/thing", () => ({ ok: true }));

    await api.handle("/api/v1/thing", {
      headers: { Authorization: "Bearer abc" },
    });

    expect(api.lastRequest()!.headers.get("authorization")).toBe("Bearer abc");
    expect(api.authTokens()).toEqual(["Bearer abc"]);
  });

  it("reports a missing Authorization header as null", async () => {
    const api = server();
    api.get("/api/v1/thing", () => ({ ok: true }));

    await api.handle("/api/v1/thing");

    expect(api.authTokens()).toEqual([null]);
  });

  it("parses a JSON body", async () => {
    const api = server();
    api.post("/api/v1/thing", () => ({}));

    await api.handle("/api/v1/thing", {
      method: "POST",
      body: JSON.stringify({ name: "x" }),
    });

    expect(api.lastRequest()!.body).toEqual({ name: "x" });
  });

  it("passes a non-JSON body through instead of throwing", async () => {
    const api = server();
    api.post("/api/v1/backup/restore", () => ({}));
    const form = new FormData();
    form.append("backup", new Blob(["data"]), "backup.db");

    await expect(
      api.handle("/api/v1/backup/restore", { method: "POST", body: form }),
    ).resolves.toBeInstanceOf(Response);
    expect(api.lastRequest()!.body).toBe(form);
  });
});

describe("apiServer responses", () => {
  it("returns a real Response supporting text()", async () => {
    const api = server();
    api.get("/api/v1/thing", () => ({ value: 1 }));

    const response = await api.handle("/api/v1/thing");

    expect(await response.text()).toBe('{"result":{"value":1}}');
  });

  it("returns a real Response supporting blob()", async () => {
    const api = server();
    api.get("/api/v1/backup", () => rawBody("binary-bytes"));

    const response = await api.handle("/api/v1/backup");

    expect((await response.blob()).size).toBeGreaterThan(0);
  });

  it("exposes response headers", async () => {
    const api = server();
    api.get("/api/v1/thing", () =>
      httpError(503, "starting", { "Retry-After": "5" }),
    );

    const response = await api.handle("/api/v1/thing");

    expect(response.status).toBe(503);
    expect(response.headers.get("retry-after")).toBe("5");
  });
});

describe("apiServer routing", () => {
  it("flags a request with no registered handler", async () => {
    const api = server();

    const response = await api.handle("/api/v1/typo");

    expect(response.status).toBe(501);
    expect(api.unhandledRequests()).toHaveLength(1);
  });

  it("does not flag a handled request", async () => {
    const api = server();
    api.get("/api/v1/thing", () => ({}));

    await api.handle("/api/v1/thing");

    expect(api.unhandledRequests()).toEqual([]);
  });

  it("replaces a route when the same path is registered again", async () => {
    const api = server();
    api.get("/api/v1/thing", () => ({ version: 1 }));
    api.get("/api/v1/thing", () => ({ version: 2 }));

    const response = await api.handle("/api/v1/thing");

    expect(await response.json()).toEqual({ result: { version: 2 } });
  });

  it("matches path parameters", async () => {
    const api = server();
    api.get("/api/v1/subscribers/:imsi", ({ url }) => ({
      imsi: url.pathname.split("/").at(-1),
    }));

    const response = await api.handle("/api/v1/subscribers/00101000");

    expect(await response.json()).toEqual({ result: { imsi: "00101000" } });
  });

  it("does not match a path with a different segment count", async () => {
    const api = server();
    api.get("/api/v1/subscribers/:imsi", () => ({}));

    const response = await api.handle("/api/v1/subscribers");

    expect(response.status).toBe(501);
  });
});
