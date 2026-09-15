// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, beforeAll, afterAll, vi } from "vitest";
import {
  CUSTOM_RANGE,
  DAILY_RANGES,
  resolveTimeRangeFilter,
  timeRangeError,
  timeRangeFilter,
  timeRangeLabel,
  toInputValue,
} from "./TimeRangePicker";

const originalTz = process.env.TZ;

beforeAll(() => {
  process.env.TZ = "America/Toronto";
});

afterAll(() => {
  process.env.TZ = originalTz;
});

describe("timeRangeLabel", () => {
  it("names a custom day range by its calendar dates", () => {
    expect(
      timeRangeLabel(
        { preset: CUSTOM_RANGE, from: "2026-09-04", to: "2026-10-23" },
        { ranges: DAILY_RANGES, granularity: "date" },
      ),
    ).toBe("Sep 4, 2026 → Oct 23, 2026");
  });

  it("names an open-ended day range by its calendar date", () => {
    expect(
      timeRangeLabel(
        { preset: CUSTOM_RANGE, from: "", to: "2026-10-23" },
        { ranges: DAILY_RANGES, granularity: "date" },
      ),
    ).toBe("Before Oct 23, 2026");
  });

  it("names a preset by its label", () => {
    expect(
      timeRangeLabel(
        { preset: "7d", from: "", to: "" },
        { ranges: DAILY_RANGES, granularity: "date" },
      ),
    ).toBe("Last 7 days");
  });
});

describe("timeRangeFilter", () => {
  it("keeps day bounds as calendar dates", () => {
    expect(
      timeRangeFilter(
        { preset: CUSTOM_RANGE, from: "2026-09-04", to: "2026-10-23" },
        "date",
      ),
    ).toEqual({ from: "2026-09-04", to: "2026-10-23" });
  });

  it("carries a preset as a relative token", () => {
    expect(timeRangeFilter({ preset: "7d", from: "", to: "" }, "date")).toEqual(
      {
        relative: "7d",
      },
    );
  });
});

describe("resolveTimeRangeFilter", () => {
  it("resolves a day preset to an inclusive calendar range", () => {
    const resolved = resolveTimeRangeFilter(
      { relative: "7d" },
      { ranges: DAILY_RANGES, granularity: "date" },
    );
    const from = new Date(`${resolved.from}T00:00:00`);
    const to = new Date(`${resolved.to}T00:00:00`);
    const days = Math.round((to.getTime() - from.getTime()) / 86_400_000) + 1;
    expect(days).toBe(7);
  });

  it("resolves yesterday to a single day", () => {
    const resolved = resolveTimeRangeFilter(
      { relative: "yesterday" },
      { ranges: DAILY_RANGES, granularity: "date" },
    );
    expect(resolved.from).toBe(resolved.to);
  });

  it("counts calendar days across a daylight saving change", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date(2026, 2, 9, 0, 30));
    try {
      expect(
        resolveTimeRangeFilter(
          { relative: "yesterday" },
          { ranges: DAILY_RANGES, granularity: "date" },
        ),
      ).toEqual({ from: "2026-03-08", to: "2026-03-08" });
      expect(
        resolveTimeRangeFilter(
          { relative: "7d" },
          { ranges: DAILY_RANGES, granularity: "date" },
        ),
      ).toEqual({ from: "2026-03-03", to: "2026-03-09" });
    } finally {
      vi.useRealTimers();
    }
  });

  it("resolves a relative instant to the past", () => {
    const resolved = resolveTimeRangeFilter({ relative: "15m" });
    const elapsed = Date.now() - Date.parse(resolved.from!);
    expect(elapsed).toBeGreaterThan(14 * 60_000);
    expect(elapsed).toBeLessThan(16 * 60_000);
  });
});

describe("toInputValue", () => {
  it("renders an instant in local time for a datetime input", () => {
    expect(toInputValue("2026-10-23T00:30:00.000Z", "datetime")).toBe(
      "2026-10-22T20:30",
    );
  });

  it("passes a calendar date through for a date input", () => {
    expect(toInputValue("2026-10-23", "date")).toBe("2026-10-23");
  });
});

describe("timeRangeError", () => {
  it("rejects an inverted range", () => {
    expect(
      timeRangeError({
        preset: CUSTOM_RANGE,
        from: "2026-10-23",
        to: "2026-09-04",
      }),
    ).toMatch(/on or after/i);
  });

  it("accepts a preset", () => {
    expect(timeRangeError({ preset: "7d", from: "", to: "" })).toBe("");
  });
});
