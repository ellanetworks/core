// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, expect, it } from "vitest";
import { describeDuration } from "@/utils/durations";

describe("describeDuration", () => {
  it("names each unit in words", () => {
    expect(describeDuration(300)).toBe("5 minutes");
    expect(describeDuration(3600)).toBe("1 hour");
    expect(describeDuration(5400)).toBe("1 hour 30 minutes");
    expect(describeDuration(86400)).toBe("1 day");
    expect(describeDuration(2592000)).toBe("30 days");
    expect(describeDuration(90061)).toBe("1 day 1 hour 1 minute 1 second");
  });
});
