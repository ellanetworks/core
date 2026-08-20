// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { afterEach, vi } from "vitest";
import { cleanup } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

process.env.TZ = "UTC";

if (!("ResizeObserver" in globalThis)) {
  globalThis.ResizeObserver = class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
}

if (!window.matchMedia) {
  window.matchMedia = (query: string) =>
    ({
      matches: false,
      media: query,
      onchange: null,
      addListener: () => {},
      removeListener: () => {},
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }) as MediaQueryList;
}

if (!globalThis.IntersectionObserver) {
  globalThis.IntersectionObserver = class {
    root = null;
    rootMargin = "";
    thresholds = [];
    observe() {}
    unobserve() {}
    disconnect() {}
    takeRecords() {
      return [];
    }
  } as unknown as typeof IntersectionObserver;
}

const consoleFailures: string[] = [];
const EXPECTED = /^expected-in-test:/;

for (const level of ["error", "warn"] as const) {
  const original = console[level];
  console[level] = (...args: unknown[]) => {
    const text = args.map(String).join(" ");
    if (!EXPECTED.test(text)) consoleFailures.push(`console.${level}: ${text}`);
    original(...args);
  };
}

export const allowConsole = () => {
  consoleFailures.length = 0;
};

afterEach(() => {
  cleanup();
  vi.useRealTimers();

  const failures = [...consoleFailures];
  consoleFailures.length = 0;
  if (failures.length > 0) {
    throw new Error(`the test logged to the console:\n${failures.join("\n")}`);
  }
});
