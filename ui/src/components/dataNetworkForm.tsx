// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import * as yup from "yup";
import TextControl from "@/components/form/TextControl";
import NumberControl from "@/components/form/NumberControl";
import {
  ipv4Regex,
  ipv6Regex,
  isValidIpv4Cidr,
  isValidIpv6PoolCidr,
} from "@/utils/ip";

export const dataNetworkNameRegex =
  /^(?=.{1,100}$)([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$/;

export const IPV6_POOL_HELPER_TEXT =
  "Prefix length between /48 and /60 — Ella Core delegates /64s from within the pool.";

export const poolAndDnsSchema = {
  ipv4_pool: yup
    .string()
    .test(
      "at-least-one-pool",
      "At least one IP pool (IPv4 or IPv6) is required",
      function (value) {
        const { ipv6_pool } = this.parent;
        return !!(value || ipv6_pool);
      },
    )
    .test(
      "ipv4-pool-format",
      "Must be a valid IPv4 CIDR (e.g., 10.45.0.0/16)",
      (value) => (value ? isValidIpv4Cidr(value) : true),
    )
    .default(""),
  ipv6_pool: yup
    .string()
    .test(
      "at-least-one-pool",
      "At least one IP pool (IPv4 or IPv6) is required",
      function (value) {
        const { ipv4_pool } = this.parent;
        return !!(value || ipv4_pool);
      },
    )
    .test(
      "ipv6-pool-format",
      "Must be a valid IPv6 CIDR with a prefix length between /48 and /60 (e.g., 2001:db8::/56)",
      (value) => (value ? isValidIpv6PoolCidr(value) : true),
    )
    .default(""),
  dns: yup
    .string()
    .test("dns-format", "Must be a valid IPv4 or IPv6 address", (value) => {
      if (!value) return false;
      return ipv4Regex.test(value) || ipv6Regex.test(value);
    })
    .required("DNS is required"),
  mtu: yup.number().min(1).max(65535).required("MTU is required"),
};

export const DataNetworkFields = ({
  autoFocusPool = false,
}: {
  autoFocusPool?: boolean;
}) => (
  <>
    <TextControl name="ipv4_pool" label="IPv4 Pool" autoFocus={autoFocusPool} />
    <TextControl
      name="ipv6_pool"
      label="IPv6 Pool"
      helperText={IPV6_POOL_HELPER_TEXT}
      placeholder="e.g., 2001:db8::/48"
    />
    <TextControl name="dns" label="DNS" />
    <NumberControl name="mtu" label="MTU" />
  </>
);
