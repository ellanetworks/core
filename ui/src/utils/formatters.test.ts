// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect, vi, afterEach } from "vitest";
import {
  buildProtocolColorMap,
  chooseUnitFromMax,
  formatBytesAutoUnit,
  formatBytesWithUnit,
  formatCountShare,
  formatDate,
  formatDateTime,
  formatMemory,
  formatProtocol,
  formatRelativeTime,
  PROTOCOL_CHIP_COLORS,
  UNIT_FACTORS,
} from "./formatters";

describe("UNIT_FACTORS", () => {
  it("is decimal, not binary", () => {
    expect(UNIT_FACTORS.KB).toBe(1000);
    expect(UNIT_FACTORS.MB).toBe(1_000_000);
    expect(UNIT_FACTORS.GB).toBe(1_000_000_000);
    expect(UNIT_FACTORS.TB).toBe(1_000_000_000_000);
  });
});

describe("chooseUnitFromMax", () => {
  it.each([
    [0, "B"],
    [999, "B"],
    [1000, "KB"],
    [999_999, "KB"],
    [1_000_000, "MB"],
    [1_000_000_000, "GB"],
    [1_000_000_000_000, "TB"],
    [5_000_000_000_000, "TB"],
  ])("%i bytes -> %s", (bytes, unit) => {
    expect(chooseUnitFromMax(bytes)).toBe(unit);
  });
});

describe("formatBytesWithUnit", () => {
  it.each([
    [1_500_000, "MB", "1.50 MB"],
    [15_000_000, "MB", "15.0 MB"],
    [150_000_000, "MB", "150 MB"],
    [0, "B", "0.00 B"],
  ] as const)("%i as %s -> %s", (bytes, unit, expected) => {
    expect(formatBytesWithUnit(bytes, unit)).toBe(expected);
  });

  it("returns empty for a non-finite value", () => {
    expect(formatBytesWithUnit(NaN, "MB")).toBe("");
    expect(formatBytesWithUnit(Infinity, "MB")).toBe("");
  });
});

describe("formatBytesAutoUnit", () => {
  it.each([
    [0, "0.00 B"],
    [512, "512 B"],
    [1000, "1.00 KB"],
    [1_500_000, "1.50 MB"],
    [2_500_000_000, "2.50 GB"],
    [3_000_000_000_000, "3.00 TB"],
  ])("%i -> %s", (bytes, expected) => {
    expect(formatBytesAutoUnit(bytes)).toBe(expected);
  });

  it("picks the unit from the magnitude, ignoring sign", () => {
    expect(formatBytesAutoUnit(-1_500_000)).toBe("-1.50 MB");
  });

  it("returns empty for a non-finite value", () => {
    expect(formatBytesAutoUnit(NaN)).toBe("");
  });
});

describe("formatMemory", () => {
  it.each([
    [0, "0 B"],
    [1024, "1 KiB"],
    [1_048_576, "1 MiB"],
    [1_073_741_824, "1 GiB"],
    [1536, "1.5 KiB"],
    [1_099_511_627_776, "1 TiB"],
  ])("%i -> %s", (value, expected) => {
    expect(formatMemory(value)).toBe(expected);
  });

  it("is N/A when absent, not 0", () => {
    expect(formatMemory(null)).toBe("N/A");
    expect(formatMemory(undefined)).toBe("N/A");
    expect(formatMemory(NaN)).toBe("N/A");
  });

  it("keeps the sign", () => {
    expect(formatMemory(-1024)).toBe("-1 KiB");
  });
});

describe("formatProtocol", () => {
  it.each([
    [6, "TCP"],
    [17, "UDP"],
    [1, "ICMP"],
  ])("%i -> %s", (num, name) => {
    expect(formatProtocol(num)).toBe(name);
  });

  it("falls back to the number when unknown", () => {
    expect(formatProtocol(253)).toBe("253");
  });
});

describe("buildProtocolColorMap", () => {
  const TCP = 6;
  const UDP = 17;
  const ICMP = 1;
  const OSPF = 89;
  const IPV6 = 41;

  it("gives a well-known protocol its colour whatever its position", () => {
    const first = buildProtocolColorMap([TCP, UDP, ICMP]);
    const second = buildProtocolColorMap([ICMP, UDP, TCP]);

    expect(first.get(TCP)).toBe(PROTOCOL_CHIP_COLORS[TCP]);
    expect(second.get(TCP)).toBe(PROTOCOL_CHIP_COLORS[TCP]);
    expect(first.get(ICMP)).toBe(second.get(ICMP));
  });

  it("does not hand an unlisted protocol a well-known protocol's colour", () => {
    const map = buildProtocolColorMap([TCP, OSPF, UDP]);

    expect(map.get(OSPF)).not.toBe(map.get(UDP));
    expect(map.get(OSPF)).not.toBe(map.get(TCP));
  });

  it("colours every protocol distinctly", () => {
    const protocols = [ICMP, IPV6, OSPF, TCP, UDP];
    const colors = [...buildProtocolColorMap(protocols).values()];

    expect(colors).toHaveLength(protocols.length);
    expect(new Set(colors).size).toBe(protocols.length);
  });

  it("has no entries for an empty set", () => {
    expect(buildProtocolColorMap([]).size).toBe(0);
  });
});

describe("formatCountShare", () => {
  it("names the unit so a flow share cannot read as a byte share", () => {
    expect(formatCountShare(1234, 10000, "flow")).toBe("1,234 flows (12.3%)");
  });

  it("singularises", () => {
    expect(formatCountShare(1, 10, "flow")).toBe("1 flow (10.0%)");
  });

  it("does not divide by zero", () => {
    expect(formatCountShare(0, 0, "flow")).toBe("0 flows (0.0%)");
  });

  it("reports a whole share as 100%", () => {
    expect(formatCountShare(5, 5, "flow")).toBe("5 flows (100.0%)");
  });
});

describe("formatDate", () => {
  it("carries the year, since expiries are read against distant dates", () => {
    expect(formatDate("2026-07-14T12:00:00Z")).toBe("Jul 14, 2026");
  });

  it("returns the input unchanged when it is not a date", () => {
    expect(formatDate("not-a-date")).toBe("not-a-date");
  });

  it("returns empty for empty", () => {
    expect(formatDate("")).toBe("");
  });
});

describe("formatDateTime", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("omits the year within the current year", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-07-14T00:00:00Z"));
    expect(formatDateTime("2026-03-08T14:32:00Z")).toBe("Mar 8, 14:32");
  });

  it("carries the year outside the current year", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-07-14T00:00:00Z"));
    expect(formatDateTime("2025-03-08T14:32:00Z")).toBe("Mar 8, 2025, 14:32");
  });

  it("is 24-hour", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-07-14T00:00:00Z"));
    expect(formatDateTime("2026-03-08T23:32:00Z")).toContain("23:32");
  });

  it("adds seconds on request", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-07-14T00:00:00Z"));
    expect(formatDateTime("2026-03-08T14:32:45Z", { seconds: true })).toBe(
      "Mar 8, 14:32:45",
    );
  });

  // V8 parses far more than it rejects — "garbage +0000" becomes the year 2000 —
  // so the fallback is reached only by a string with nothing date-like in it.
  it("returns an unparseable string unchanged", () => {
    expect(formatDateTime("not-a-date")).toBe("not-a-date");
  });

  it("returns empty for empty", () => {
    expect(formatDateTime("")).toBe("");
  });
});

describe("formatRelativeTime", () => {
  const now = new Date("2026-07-14T12:00:00Z");

  afterEach(() => {
    vi.useRealTimers();
  });

  const at = (iso: string) => {
    vi.useFakeTimers();
    vi.setSystemTime(now);
    return formatRelativeTime(iso);
  };

  it.each([
    ["2026-07-14T11:59:30Z", "30s ago"],
    ["2026-07-14T11:30:00Z", "30m ago"],
    ["2026-07-14T09:00:00Z", "3h ago"],
    ["2026-07-11T12:00:00Z", "3d ago"],
  ])("%s -> %s", (iso, expected) => {
    expect(at(iso)).toBe(expected);
  });

  it.each([
    ["2026-07-14T11:59:00Z", "60s boundary", "1m ago"],
    ["2026-07-14T11:00:00Z", "60m boundary", "1h ago"],
    ["2026-07-13T12:00:00Z", "24h boundary", "1d ago"],
  ])("%s (%s) -> %s", (iso, _label, expected) => {
    expect(at(iso)).toBe(expected);
  });

  // A clock skew between server and browser must not render "-5s ago".
  it("reports a future timestamp as just now", () => {
    expect(at("2026-07-14T12:00:05Z")).toBe("just now");
  });
});
