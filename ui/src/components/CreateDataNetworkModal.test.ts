// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { describe, it, expect } from "vitest";
import { ValidationError } from "yup";
import { schema } from "./CreateDataNetworkModal";

const NEED_ONE = "At least one IP pool (IPv4 or IPv6) is required";
const V4_FORMAT = "Must be a valid IPv4 CIDR (e.g., 10.45.0.0/16)";
const V6_FORMAT =
  "Must be a valid IPv6 CIDR with a prefix length between /48 and /60 (e.g., 2001:db8::/56)";

const base = {
  name: "internet",
  ipv4_pool: "10.45.0.0/16",
  ipv6_pool: "",
  dns: "8.8.8.8",
  mtu: 1500,
};

/** The messages yup raises for a form, or [] when it validates. */
const errorsFor = async (form: Record<string, unknown>): Promise<string[]> => {
  try {
    await schema.validate(form, { abortEarly: false });
    return [];
  } catch (err) {
    return err instanceof ValidationError ? err.errors : [String(err)];
  }
};

describe("data network pool validation", () => {
  it("accepts an IPv4 pool alone", async () => {
    expect(await errorsFor(base)).toEqual([]);
  });

  it("accepts an IPv6 pool alone", async () => {
    expect(
      await errorsFor({ ...base, ipv4_pool: "", ipv6_pool: "2001:db8::/56" }),
    ).toEqual([]);
  });

  it("accepts both", async () => {
    expect(await errorsFor({ ...base, ipv6_pool: "2001:db8::/56" })).toEqual(
      [],
    );
  });

  it("asks for a pool only when neither is given", async () => {
    const errors = await errorsFor({ ...base, ipv4_pool: "", ipv6_pool: "" });

    expect(errors).toContain(NEED_ONE);
  });

  // The finding this covers: one message for three causes meant a malformed
  // pool was reported as an absent one, with the pool visibly on screen.
  it("reports a malformed IPv6 pool as a format error, not an absent pool", async () => {
    const errors = await errorsFor({ ...base, ipv6_pool: "2001:db8::/64" });

    expect(errors).toContain(V6_FORMAT);
    expect(errors).not.toContain(NEED_ONE);
  });

  it("reports a malformed IPv4 pool as a format error, not an absent pool", async () => {
    const errors = await errorsFor({ ...base, ipv4_pool: "10.0.0.0/99" });

    expect(errors).toContain(V4_FORMAT);
    expect(errors).not.toContain(NEED_ONE);
  });

  // The pool prefix must leave room for the /64s Ella Core delegates from it:
  // isIPv6PoolValid, internal/api/server/api_data_networks.go.
  it.each([48, 52, 56, 60])("accepts a /%i IPv6 pool", async (len) => {
    expect(
      await errorsFor({ ...base, ipv6_pool: `2001:db8::/${len}` }),
    ).toEqual([]);
  });

  it.each([47, 61, 64, 128])("rejects a /%i IPv6 pool", async (len) => {
    expect(
      await errorsFor({ ...base, ipv6_pool: `2001:db8::/${len}` }),
    ).toContain(V6_FORMAT);
  });

  it.each(["banana/24", "10.0.0.0/33", "10.0.0.0", "256.0.0.0/8"])(
    "rejects %s as an IPv4 pool",
    async (pool) => {
      expect(await errorsFor({ ...base, ipv4_pool: pool })).toContain(
        V4_FORMAT,
      );
    },
  );

  // The field is labelled "IPv4 Pool" and IPv6 has its own field.
  it("does not accept an IPv6 prefix in the IPv4 pool", async () => {
    expect(await errorsFor({ ...base, ipv4_pool: "2001:db8::/56" })).toContain(
      V4_FORMAT,
    );
  });
});

describe("data network dns validation", () => {
  it.each(["8.8.8.8", "2001:4860:4860::8888"])("accepts %s", async (dns) => {
    expect(await errorsFor({ ...base, dns })).toEqual([]);
  });

  it.each(["", "banana", "256.0.0.1"])("rejects %s", async (dns) => {
    expect(await errorsFor({ ...base, dns })).not.toEqual([]);
  });
});
