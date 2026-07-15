// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { detectPreset, getImportPolicyLabel } from "./bgp";
import type { BGPImportPrefix } from "@/queries/bgp";

const p = (prefix: string, maxLength: number): BGPImportPrefix => ({
  prefix,
  maxLength,
});

describe("detectPreset", () => {
  it("reads an empty list as none", () => {
    expect(detectPreset([])).toBe("none");
  });

  // maxLength 0 accepts only the default route itself; the family width accepts
  // anything under it.
  it.each([
    [p("0.0.0.0/0", 0), "default-route"],
    [p("::/0", 0), "default-route"],
    [p("0.0.0.0/0", 32), "all"],
    [p("::/0", 128), "all"],
  ] as const)("reads %o as %s", (prefix, expected) => {
    expect(detectPreset([prefix])).toBe(expected);
  });

  const customCases: [string, BGPImportPrefix[]][] = [
    ["a specific prefix", [p("10.0.0.0/8", 24)]],
    ["the default route at a partial maxLength", [p("0.0.0.0/0", 24)]],
    ["an IPv4 default route at the IPv6 width", [p("0.0.0.0/0", 128)]],
    ["an IPv6 default route at the IPv4 width", [p("::/0", 32)]],
    ["both families at once", [p("0.0.0.0/0", 0), p("::/0", 0)]],
  ];

  it.each(customCases)("reads %s as custom", (_label, prefixes) => {
    expect(detectPreset(prefixes)).toBe("custom");
  });
});

describe("getImportPolicyLabel", () => {
  it.each([
    [undefined, "Deny All"],
    [[], "Deny All"],
  ] as const)("labels %o as %s", (prefixes, expected) => {
    expect(getImportPolicyLabel(prefixes ? [...prefixes] : undefined)).toBe(
      expected,
    );
  });

  it.each([
    [p("0.0.0.0/0", 0), "Default Route Only"],
    [p("::/0", 0), "Default Route Only"],
    [p("0.0.0.0/0", 32), "Accept All"],
    [p("::/0", 128), "Accept All"],
  ] as const)("labels %o as %s", (prefix, expected) => {
    expect(getImportPolicyLabel([prefix])).toBe(expected);
  });

  it("does not claim a preset for a list it cannot name", () => {
    expect(getImportPolicyLabel([p("10.0.0.0/8", 24)])).not.toBe("Deny All");
    expect(getImportPolicyLabel([p("10.0.0.0/8", 24)])).not.toBe("Accept All");
  });
});
