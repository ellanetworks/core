// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, act, cleanup } from "@testing-library/react";
import { useNow } from "./useNow";

const Clock = ({ interval = 1000 }: { interval?: number }) => (
  <span data-testid="now">{useNow(interval)}</span>
);

const now = () => Number(screen.getByTestId("now").textContent);

beforeEach(() => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-08-24T12:00:00.000Z"));
});

afterEach(() => {
  vi.useRealTimers();
});

describe("useNow", () => {
  it("starts on the current instant floored to the interval", () => {
    vi.setSystemTime(new Date("2026-08-24T12:00:00.750Z"));
    render(<Clock />);

    expect(now()).toBe(new Date("2026-08-24T12:00:00.000Z").getTime());
  });

  it("advances once the interval elapses", () => {
    render(<Clock />);
    const before = now();

    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(now()).toBe(before + 1000);
  });

  it("holds steady between ticks", () => {
    render(<Clock />);
    const before = now();

    act(() => {
      vi.advanceTimersByTime(999);
    });

    expect(now()).toBe(before);
  });

  it("clears its interval on unmount", () => {
    const clear = vi.spyOn(globalThis, "clearInterval");
    const set = vi.spyOn(globalThis, "setInterval");
    render(<Clock />);
    const opened = set.mock.calls.length;

    cleanup();

    expect(clear.mock.calls.length).toBe(opened);
    clear.mockRestore();
    set.mockRestore();
  });
});
