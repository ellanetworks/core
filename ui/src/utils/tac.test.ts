// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { TAC_HEX_PATTERN, formatTacDecimal, tacToDecimal } from "./tac";

describe("tacToDecimal", () => {
  it.each([
    ["000000", 0],
    ["000001", 1],
    ["000051", 81],
    ["00ffff", 65535],
    ["ffffff", 16777215],
    ["00FFFF", 65535],
  ])("%s -> %i", (tac, decimal) => {
    expect(tacToDecimal(tac)).toBe(decimal);
  });

  it("accepts the short forms the core tolerates", () => {
    expect(tacToDecimal("1")).toBe(1);
    expect(tacToDecimal("51")).toBe(81);
  });

  it("ignores surrounding whitespace", () => {
    expect(tacToDecimal("  000051  ")).toBe(81);
  });

  it.each([
    ["", "empty"],
    ["0000051", "wider than three octets"],
    ["00005g", "not hex"],
    ["0x0051", "hex prefixed"],
    ["-00051", "signed"],
  ])("rejects %s (%s)", (tac) => {
    expect(tacToDecimal(tac)).toBeNull();
  });
});

describe("formatTacDecimal", () => {
  it("renders the decimal a radio would be configured with", () => {
    expect(formatTacDecimal("000051")).toBe("81");
  });

  it("returns null for a value it cannot read, so callers show the raw TAC", () => {
    expect(formatTacDecimal("nope")).toBeNull();
  });
});

describe("TAC_HEX_PATTERN", () => {
  it("matches the six-digit form the API requires", () => {
    expect(TAC_HEX_PATTERN.test("000051")).toBe(true);
    expect(TAC_HEX_PATTERN.test("FFFFFF")).toBe(true);
  });

  it("rejects the short forms the API rejects", () => {
    expect(TAC_HEX_PATTERN.test("51")).toBe(false);
    expect(TAC_HEX_PATTERN.test("0000051")).toBe(false);
  });
});
