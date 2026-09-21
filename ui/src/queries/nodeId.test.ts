// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { nodeLabel, shortNodeId } from "./nodeId";

const NODE_A = "0199c4f1-2ab3-7c1d-9f2a-6fe8422da1ec";
const NODE_B = "0199c4f1-2ab4-7e55-8b01-3a77c90ee412";

describe("shortNodeId", () => {
  it("distinguishes UUIDv7 ids minted in the same timestamp window", () => {
    expect(NODE_A.slice(0, 8)).toBe(NODE_B.slice(0, 8));
    expect(shortNodeId(NODE_A)).not.toBe(shortNodeId(NODE_B));
  });

  it("keeps the trailing random group", () => {
    expect(shortNodeId(NODE_A)).toBe("6fe8422da1ec");
    expect(shortNodeId(NODE_B)).toBe("3a77c90ee412");
  });

  it("returns legacy integer ids unchanged", () => {
    expect(shortNodeId(2)).toBe("2");
    expect(shortNodeId("63")).toBe("63");
  });

  it("returns an empty string when there is no id", () => {
    expect(shortNodeId(undefined)).toBe("");
  });
});

describe("nodeLabel", () => {
  it("prefers the display name", () => {
    expect(nodeLabel("core-1", NODE_A)).toBe("core-1");
  });

  it("falls back to the short id", () => {
    expect(nodeLabel("", NODE_A)).toBe("6fe8422da1ec");
    expect(nodeLabel(undefined, NODE_A)).toBe("6fe8422da1ec");
  });
});
