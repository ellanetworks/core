// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

let onUnauthorized: (() => void) | null = null;

export function setOnUnauthorized(cb: (() => void) | null) {
  onUnauthorized = cb;
}

export type ApiErrorKind = "network" | "auth" | "forbidden" | "http";

/**
 * Carries the failure shape the UI needs to choose a state, so callers never
 * have to parse a message string to tell "unreachable" from "500" from "403".
 *
 * `message` is the human-readable primary line; `detail` is the technical line
 * for progressive disclosure.
 */
export class ApiError extends Error {
  readonly kind: ApiErrorKind;
  readonly status?: number;
  readonly detail?: string;

  constructor(
    kind: ApiErrorKind,
    message: string,
    options: { status?: number; detail?: string } = {},
  ) {
    super(message);
    this.name = "ApiError";
    this.kind = kind;
    this.status = options.status;
    this.detail = options.detail;
    Object.setPrototypeOf(this, ApiError.prototype);
  }

  get retryable(): boolean {
    if (this.kind === "network") return true;
    return this.status !== undefined && this.status >= 500;
  }
}

const toApiError = (status: number, backendMessage?: string): ApiError => {
  const detail = `HTTP ${status} ${HTTPStatus(status)}`;
  if (status === 401) {
    return new ApiError(
      "auth",
      backendMessage || "Your session has expired. Please log in again.",
      { status, detail },
    );
  }
  if (status === 403) {
    return new ApiError(
      "forbidden",
      backendMessage || "You do not have permission to perform this action.",
      { status, detail },
    );
  }
  return new ApiError("http", backendMessage || HTTPStatus(status), {
    status,
    detail,
  });
};

export const HTTPStatus = (code: number): string => {
  const map: { [key: number]: string } = {
    400: "Bad Request",
    401: "Unauthorized",
    403: "Forbidden",
    404: "Not Found",
    409: "Conflict",
    413: "Payload Too Large",
    422: "Unprocessable Entity",
    429: "Too Many Requests",
    500: "Internal Server Error",
    502: "Bad Gateway",
    503: "Service Unavailable",
    504: "Gateway Timeout",
  };
  return map[code] ?? `HTTP Error ${code}`;
};

interface ApiFetchOptions {
  method?: string;
  authToken?: string;
  body?: unknown;
  credentials?: RequestCredentials;
}

export async function apiFetch<T = unknown>(
  url: string,
  options: ApiFetchOptions = {},
): Promise<T> {
  const { method = "GET", authToken, body, credentials } = options;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (authToken) {
    headers["Authorization"] = "Bearer " + authToken;
  }

  const init: RequestInit = { method, headers };
  if (body !== undefined) {
    init.body = JSON.stringify(body);
  }
  if (credentials) {
    init.credentials = credentials;
  }

  let response: Response;
  try {
    response = await fetch(url, init);
  } catch {
    throw new ApiError("network", "Cannot reach the server.", {
      detail: `Network request to ${url} failed`,
    });
  }

  if (response.status === 401) {
    onUnauthorized?.();
  }

  let respData: { result?: T; error?: string } | undefined;
  try {
    respData = await response.json();
  } catch {
    if (!response.ok) {
      throw toApiError(response.status, response.statusText || undefined);
    }
    return undefined as T;
  }

  if (!response.ok) {
    throw toApiError(response.status, respData?.error);
  }

  return respData!.result as T;
}

export async function apiFetchVoid(
  url: string,
  options: ApiFetchOptions = {},
): Promise<void> {
  const { method = "GET", authToken, body, credentials } = options;

  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (authToken) {
    headers["Authorization"] = "Bearer " + authToken;
  }

  const init: RequestInit = { method, headers };
  if (body !== undefined) {
    init.body = JSON.stringify(body);
  }
  if (credentials) {
    init.credentials = credentials;
  }

  let response: Response;
  try {
    response = await fetch(url, init);
  } catch {
    throw new ApiError("network", "Cannot reach the server.", {
      detail: `Network request to ${url} failed`,
    });
  }

  if (response.status === 401) {
    onUnauthorized?.();
  }

  if (!response.ok) {
    let respData: { error?: string } | undefined;
    try {
      respData = await response.json();
    } catch {
      throw toApiError(response.status, response.statusText || undefined);
    }
    throw toApiError(response.status, respData?.error);
  }
}
