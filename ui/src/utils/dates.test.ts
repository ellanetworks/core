// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, afterEach } from "vitest";
import { startOfLocalDay } from "./dates";

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
