// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, afterEach } from "vitest";
import { localDateString, defaultDateRange } from "./dates";

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

describe("localDateString", () => {
  it("reports the local calendar date east of UTC when UTC is still on the previous day", () => {
    const instant = new Date("2026-08-18T15:30:00Z");

    expect(inZone("Asia/Tokyo", () => localDateString(instant))).toBe(
      "2026-08-19",
    );
    expect(inZone("UTC", () => localDateString(instant))).toBe("2026-08-18");
  });

  it("reports the local calendar date west of UTC when UTC has already rolled over", () => {
    const instant = new Date("2026-08-19T03:30:00Z");

    expect(inZone("America/New_York", () => localDateString(instant))).toBe(
      "2026-08-18",
    );
    expect(inZone("UTC", () => localDateString(instant))).toBe("2026-08-19");
  });

  it("zero-pads single-digit months and days", () => {
    expect(
      inZone("UTC", () => localDateString(new Date("2026-03-07T12:00:00Z"))),
    ).toBe("2026-03-07");
  });
});

describe("defaultDateRange", () => {
  it("ends on the viewer's local today, not the UTC date", () => {
    const instant = new Date("2026-08-18T15:30:00Z");

    expect(inZone("Asia/Tokyo", () => defaultDateRange(7, instant))).toEqual({
      startDate: "2026-08-13",
      endDate: "2026-08-19",
    });
    expect(
      inZone("America/New_York", () => defaultDateRange(7, instant)),
    ).toEqual({ startDate: "2026-08-12", endDate: "2026-08-18" });
  });

  it("covers an inclusive window of the requested width", () => {
    const { startDate, endDate } = inZone("UTC", () =>
      defaultDateRange(7, new Date("2026-08-18T12:00:00Z")),
    );

    expect(startDate).toBe("2026-08-12");
    expect(endDate).toBe("2026-08-18");
  });

  it("rolls back across a month boundary", () => {
    expect(
      inZone("UTC", () =>
        defaultDateRange(7, new Date("2026-03-03T12:00:00Z")),
      ),
    ).toEqual({ startDate: "2026-02-25", endDate: "2026-03-03" });
  });

  it("rolls back across a year boundary", () => {
    expect(
      inZone("UTC", () =>
        defaultDateRange(7, new Date("2026-01-03T12:00:00Z")),
      ),
    ).toEqual({ startDate: "2025-12-28", endDate: "2026-01-03" });
  });

  it("stays on the local day across a DST spring-forward transition", () => {
    expect(
      inZone("America/New_York", () =>
        defaultDateRange(7, new Date("2026-03-09T12:00:00Z")),
      ),
    ).toEqual({ startDate: "2026-03-03", endDate: "2026-03-09" });
  });
});
