// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, afterEach } from "vitest";
import { defaultDateRange, startOfLocalDay } from "./dates";

const ORIGINAL_TZ = process.env.TZ;

const inZone = <T>(tz: string, fn: () => T): T => {
  process.env.TZ = tz;
  try {
    return fn();
  } finally {
    process.env.TZ = ORIGINAL_TZ;
  }
};

afterEach(() => {
  process.env.TZ = ORIGINAL_TZ;
});

describe("startOfLocalDay", () => {
  it("returns local midnight, not UTC midnight", () => {
    const at = inZone("Asia/Tokyo", () =>
      startOfLocalDay(0, new Date("2026-08-18T15:30:00Z")),
    );

    expect(inZone("Asia/Tokyo", () => at.getDate())).toBe(19);
    expect(inZone("Asia/Tokyo", () => at.getHours())).toBe(0);
  });
});

describe("defaultDateRange", () => {
  const localDay = (tz: string, iso: string) =>
    inZone(tz, () => {
      const d = new Date(iso);
      const pad = (n: number) => String(n).padStart(2, "0");
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
    });

  it("starts at local midnight of the first day in the window", () => {
    const instant = new Date("2026-08-18T15:30:00Z");

    const tokyo = inZone("Asia/Tokyo", () => defaultDateRange(7, instant));
    expect(localDay("Asia/Tokyo", tokyo.startDate)).toBe("2026-08-13");

    const newYork = inZone("America/New_York", () =>
      defaultDateRange(7, instant),
    );
    expect(localDay("America/New_York", newYork.startDate)).toBe("2026-08-12");
  });

  it("ends at the instant it was called", () => {
    const instant = new Date("2026-08-18T15:30:00Z");

    expect(inZone("UTC", () => defaultDateRange(7, instant)).endDate).toBe(
      instant.toISOString(),
    );
  });

  it("rolls back across a month boundary", () => {
    const { startDate } = inZone("UTC", () =>
      defaultDateRange(7, new Date("2026-03-03T12:00:00Z")),
    );

    expect(localDay("UTC", startDate)).toBe("2026-02-25");
  });

  it("rolls back across a year boundary", () => {
    const { startDate } = inZone("UTC", () =>
      defaultDateRange(7, new Date("2026-01-03T12:00:00Z")),
    );

    expect(localDay("UTC", startDate)).toBe("2025-12-28");
  });

  it("stays on the local day across a DST spring-forward transition", () => {
    const { startDate } = inZone("America/New_York", () =>
      defaultDateRange(7, new Date("2026-03-09T12:00:00Z")),
    );

    expect(localDay("America/New_York", startDate)).toBe("2026-03-03");
  });
});
