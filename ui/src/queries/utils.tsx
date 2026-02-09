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

/**
 * Shared fetch wrapper for JSON API endpoints.
 * Handles auth headers, JSON parsing, and error formatting.
 * Returns `response.result` from the JSON body.
 */
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

  const response = await fetch(url, init);

  let respData: { result?: T; error?: string } | undefined;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData!.result as T;
}

/**
 * Variant of apiFetch for endpoints that return no meaningful body.
 * Throws on error, returns void on success.
 */
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

  const response = await fetch(url, init);

  if (!response.ok) {
    let respData: { error?: string } | undefined;
    try {
      respData = await response.json();
    } catch {
      throw new Error(
        `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
      );
    }
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }
}
