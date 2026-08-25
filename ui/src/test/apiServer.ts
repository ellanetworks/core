// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { vi, beforeEach, afterEach } from "vitest";

export type ApiRequest = {
  method: string;
  url: URL;
  params: URLSearchParams;
  headers: Headers;
  body: unknown;
};

export type Resolver = (request: ApiRequest) => unknown | Promise<unknown>;

export class ApiFailure {
  constructor(
    readonly status: number,
    readonly error?: string,
    readonly headers: Record<string, string> = {},
  ) {}
}

export const httpError = (
  status: number,
  error?: string,
  headers?: Record<string, string>,
): ApiFailure => new ApiFailure(status, error, headers);

export class RawBody {
  constructor(
    readonly body: BodyInit,
    readonly headers: Record<string, string> = {},
  ) {}
}

export const rawBody = (body: BodyInit, headers?: Record<string, string>) =>
  new RawBody(body, headers);

type Route = {
  method: string;
  path: string;
  segments: string[];
  resolver: Resolver;
};

const segmentsOf = (path: string) => path.replace(/^\/+|\/+$/g, "").split("/");

const matches = (route: Route, method: string, pathname: string) => {
  if (route.method !== method) return false;
  const actual = segmentsOf(pathname);
  if (actual.length !== route.segments.length) return false;
  return route.segments.every(
    (segment, i) => segment.startsWith(":") || segment === actual[i],
  );
};

const parseBody = (body: BodyInit | null | undefined): unknown => {
  if (body == null) return undefined;
  if (typeof body !== "string") return body;
  try {
    return JSON.parse(body);
  } catch {
    return body;
  }
};

const respond = (outcome: unknown): Response => {
  if (outcome instanceof ApiFailure) {
    return new Response(JSON.stringify({ error: outcome.error }), {
      status: outcome.status,
      headers: { "Content-Type": "application/json", ...outcome.headers },
    });
  }
  if (outcome instanceof RawBody) {
    return new Response(outcome.body, {
      status: 200,
      headers: outcome.headers,
    });
  }
  if (outcome === undefined) {
    return new Response(null, { status: 204 });
  }
  return new Response(JSON.stringify({ result: outcome }), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
};

export class ApiServer {
  private routes: Route[] = [];
  private calls: ApiRequest[] = [];
  private unhandled: ApiRequest[] = [];

  on(method: string, path: string, resolver: Resolver): this {
    const route: Route = {
      method: method.toUpperCase(),
      path,
      segments: segmentsOf(path),
      resolver,
    };
    const existing = this.routes.findIndex(
      (r) => r.method === route.method && r.path === route.path,
    );
    if (existing === -1) this.routes.push(route);
    else this.routes[existing] = route;
    return this;
  }

  get = (path: string, resolver: Resolver) => this.on("GET", path, resolver);
  put = (path: string, resolver: Resolver) => this.on("PUT", path, resolver);
  post = (path: string, resolver: Resolver) => this.on("POST", path, resolver);
  patch = (path: string, resolver: Resolver) =>
    this.on("PATCH", path, resolver);
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

  authTokens(path?: string): (string | null)[] {
    return this.requests(path).map((call) => call.headers.get("authorization"));
  }

  unhandledRequests(): ApiRequest[] {
    return [...this.unhandled];
  }

  reset(): void {
    this.routes = [];
    this.calls = [];
    this.unhandled = [];
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
      headers: new Headers(init?.headers),
      body: parseBody(init?.body),
    };
    this.calls.push(request);

    const route = this.routes.find((r) => matches(r, method, url.pathname));
    if (!route) {
      this.unhandled.push(request);
      return new Response(
        JSON.stringify({ error: `no handler for ${method} ${url.pathname}` }),
        { status: 501, headers: { "Content-Type": "application/json" } },
      );
    }

    return respond(await route.resolver(request));
  };
}

export function setupApiServer(): ApiServer {
  const server = new ApiServer();

  beforeEach(() => {
    server.reset();
    vi.stubGlobal("fetch", vi.fn(server.handle));
  });

  afterEach(() => {
    const unhandled = server.unhandledRequests();
    vi.unstubAllGlobals();
    if (unhandled.length > 0) {
      const list = unhandled
        .map((r) => `  ${r.method} ${r.url.pathname}${r.url.search}`)
        .join("\n");
      throw new Error(
        `the test made requests with no registered handler:\n${list}`,
      );
    }
  });

  return server;
}
