// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { vi, beforeEach, afterEach } from "vitest";

export type ApiRequest = {
  method: string;
  url: URL;
  params: URLSearchParams;
  body: unknown;
};

export type Resolver = (request: ApiRequest) => unknown;

export class ApiFailure {
  constructor(
    readonly status: number,
    readonly error?: string,
  ) {}
}

export const httpError = (status: number, error?: string): ApiFailure =>
  new ApiFailure(status, error);

type Route = { method: string; segments: string[]; resolver: Resolver };

const segmentsOf = (path: string) => path.replace(/^\/+|\/+$/g, "").split("/");

const matches = (route: Route, method: string, pathname: string) => {
  if (route.method !== method) return false;
  const actual = segmentsOf(pathname);
  if (actual.length !== route.segments.length) return false;
  return route.segments.every(
    (segment, i) => segment.startsWith(":") || segment === actual[i],
  );
};

const respond = (outcome: unknown): Response => {
  if (outcome instanceof ApiFailure) {
    return {
      status: outcome.status,
      ok: false,
      statusText: "",
      json: async () => ({ error: outcome.error }),
    } as Response;
  }
  return {
    status: 200,
    ok: true,
    statusText: "OK",
    json: async () => ({ result: outcome }),
  } as Response;
};

export class ApiServer {
  private routes: Route[] = [];
  private calls: ApiRequest[] = [];

  on(method: string, path: string, resolver: Resolver): this {
    this.routes.push({
      method: method.toUpperCase(),
      segments: segmentsOf(path),
      resolver,
    });
    return this;
  }

  get = (path: string, resolver: Resolver) => this.on("GET", path, resolver);
  put = (path: string, resolver: Resolver) => this.on("PUT", path, resolver);
  post = (path: string, resolver: Resolver) => this.on("POST", path, resolver);
  delete = (path: string, resolver: Resolver) =>
    this.on("DELETE", path, resolver);

  requests(path?: string): ApiRequest[] {
    return path === undefined
      ? [...this.calls]
      : this.calls.filter((call) => call.url.pathname === path);
  }

  lastRequest(path?: string): ApiRequest | undefined {
    return this.requests(path).at(-1);
  }

  reset(): void {
    this.routes = [];
    this.calls = [];
  }

  handle = async (
    input: RequestInfo | URL,
    init?: RequestInit,
  ): Promise<Response> => {
    const url = new URL(String(input), window.location.origin);
    const method = (init?.method ?? "GET").toUpperCase();
    const request: ApiRequest = {
      method,
      url,
      params: url.searchParams,
      body: init?.body ? JSON.parse(String(init.body)) : undefined,
    };
    this.calls.push(request);

    for (let i = this.routes.length - 1; i >= 0; i--) {
      if (matches(this.routes[i], method, url.pathname)) {
        return respond(this.routes[i].resolver(request));
      }
    }

    throw new Error(
      `no handler registered for ${method} ${url.pathname}${url.search}`,
    );
  };
}

export function setupApiServer(): ApiServer {
  const server = new ApiServer();

  beforeEach(() => {
    server.reset();
    vi.stubGlobal("fetch", vi.fn(server.handle));
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  return server;
}
