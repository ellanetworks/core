// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

const KEY = "core.returnPath";
const MAX_AGE_MS = 10 * 60_000;

const usable = (path: unknown): path is string =>
  typeof path === "string" &&
  path.startsWith("/") &&
  !path.startsWith("//") &&
  !path.startsWith("/\\") &&
  path !== "/" &&
  !path.startsWith("/login");

export function rememberReturnPath(path: string) {
  if (!usable(path)) return;
  try {
    sessionStorage.setItem(KEY, JSON.stringify({ path, at: Date.now() }));
  } catch {
    return;
  }
}

export function returnPath(): string | undefined {
  try {
    const saved = JSON.parse(sessionStorage.getItem(KEY) ?? "null") as {
      path?: unknown;
      at?: unknown;
    } | null;
    if (
      saved &&
      usable(saved.path) &&
      typeof saved.at === "number" &&
      Date.now() - saved.at < MAX_AGE_MS
    ) {
      return saved.path;
    }
  } catch {
    return undefined;
  }
  return undefined;
}

export function forgetReturnPath() {
  try {
    sessionStorage.removeItem(KEY);
  } catch {
    return;
  }
}
