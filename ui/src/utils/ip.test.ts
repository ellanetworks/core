// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import {
  getMaxPrefixLength,
  IPV6_POOL_MAX_PREFIX,
  IPV6_POOL_MIN_PREFIX,
  ipRegex,
  ipv4Regex,
  ipv6Regex,
  isValidCidr,
  isValidIpv4Cidr,
  isValidIpv6Cidr,
  isValidIpv6PoolCidr,
  prefixLength,
} from "./ip";

describe("prefixLength", () => {
  it.each([
    ["10.0.0.0/24", 24],
    ["10.0.0.0/0", 0],
    ["2001:db8::/56", 56],
    ["::/0", 0],
  ])("reads %s as /%i", (value, expected) => {
    expect(prefixLength(value)).toBe(expected);
  });

  it.each(["10.0.0.0", "banana", "10.0.0.0/", "10.0.0.0/abc", "10.0.0.0/1234"])(
    "returns null for %s",
    (value) => {
      expect(prefixLength(value)).toBeNull();
    },
  );
});

describe("isValidIpv4Cidr", () => {
  it.each(["10.0.0.0/24", "0.0.0.0/0", "192.168.1.0/32", "255.255.255.255/32"])(
    "accepts %s",
    (value) => {
      expect(isValidIpv4Cidr(value)).toBe(true);
    },
  );

  it.each([
    ["10.0.0.0/33", "prefix above the family width"],
    ["10.0.0.0/99", "prefix far above the family width"],
    ["256.0.0.0/8", "octet out of range"],
    ["10.0.0.0", "no prefix"],
    ["banana/24", "address is not an address"],
    ["2001:db8::/56", "IPv6 is not IPv4"],
  ])("rejects %s (%s)", (value) => {
    expect(isValidIpv4Cidr(value)).toBe(false);
  });
});

describe("isValidIpv6Cidr", () => {
  it.each(["2001:db8::/56", "::/0", "2001:db8::/128", "fe80::/10"])(
    "accepts %s",
    (value) => {
      expect(isValidIpv6Cidr(value)).toBe(true);
    },
  );

  it.each(["2001:db8::/129", "2001:db8::/999", "2001:db8::", "10.0.0.0/24"])(
    "rejects %s",
    (value) => {
      expect(isValidIpv6Cidr(value)).toBe(false);
    },
  );
});

describe("isValidCidr", () => {
  // Empty is the "not filled in yet" case; required-ness is the caller's rule.
  it("treats empty as valid", () => {
    expect(isValidCidr("")).toBe(true);
  });

  it.each(["10.0.0.0/24", "0.0.0.0/0", "2001:db8::/56", "::/0"])(
    "accepts %s",
    (value) => {
      expect(isValidCidr(value)).toBe(true);
    },
  );

  it.each([
    "banana/24",
    "10.0.0.0/99",
    "2001:db8::/999",
    "10.0.0.0/33",
    "2001:db8::/129",
    "10.0.0.0",
    "/24",
    "10.0.0.0/24/8",
  ])("rejects %s", (value) => {
    expect(isValidCidr(value)).toBe(false);
  });
});

describe("isValidIpv6PoolCidr", () => {
  // Ella Core delegates /64s from within the pool, so the pool must be wider:
  // isIPv6PoolValid, internal/api/server/api_data_networks.go.
  it.each([IPV6_POOL_MIN_PREFIX, 52, 56, IPV6_POOL_MAX_PREFIX])(
    "accepts a /%i pool",
    (len) => {
      expect(isValidIpv6PoolCidr(`2001:db8::/${len}`)).toBe(true);
    },
  );

  it.each([0, 32, 47, 61, 64, 128])("rejects a /%i pool", (len) => {
    expect(isValidIpv6PoolCidr(`2001:db8::/${len}`)).toBe(false);
  });

  it("rejects an IPv4 pool", () => {
    expect(isValidIpv6PoolCidr("10.0.0.0/24")).toBe(false);
  });

  it("treats empty as valid", () => {
    expect(isValidIpv6PoolCidr("")).toBe(true);
  });
});

describe("getMaxPrefixLength", () => {
  it("is the family width", () => {
    expect(getMaxPrefixLength("10.0.0.0/16")).toBe(32);
    expect(getMaxPrefixLength("2001:db8::/56")).toBe(128);
  });
});

describe("address regexes", () => {
  it.each(["10.0.0.1", "0.0.0.0", "255.255.255.255"])(
    "ipv4 accepts %s",
    (v) => {
      expect(ipv4Regex.test(v)).toBe(true);
    },
  );

  it.each(["256.0.0.1", "10.0.0", "banana", "10.0.0.1/24"])(
    "ipv4 rejects %s",
    (v) => {
      expect(ipv4Regex.test(v)).toBe(false);
    },
  );

  it.each(["2001:db8::1", "::1", "::"])("ipv6 accepts %s", (v) => {
    expect(ipv6Regex.test(v)).toBe(true);
  });

  it("ipRegex accepts either family", () => {
    expect(ipRegex.test("10.0.0.1")).toBe(true);
    expect(ipRegex.test("2001:db8::1")).toBe(true);
  });
});
