// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

export const ipv4Regex =
  /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;

export const ipv6Regex =
  /^((([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4})|(([0-9a-fA-F]{1,4}:){1,7}:)|(([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4})|(([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2})|(([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3})|(([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4})|(([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5})|([0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6}))|(:((:[0-9a-fA-F]{1,4}){1,7}|:))|fe80:(:[0-9a-fA-F]{0,4}){0,4}%?[0-9a-fA-F]{0,4}|::(ffff(:0{1,4}){0,1}:){0,1}((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])|([0-9a-fA-F]{1,4}:){1,4}:((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9]))$/;

export const ipRegex = new RegExp(
  `(${ipv4Regex.source})|(${ipv6Regex.source})`,
);

export const cidrRegex =
  /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}\/\d{1,2}$/;

export const ipv6CidrRegex =
  /^(?:(?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|(?:[0-9a-fA-F]{1,4}:){1,7}:|(?:[0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|(?:[0-9a-fA-F]{1,4}:){1,5}(?::[0-9a-fA-F]{1,4}){1,2}|(?:[0-9a-fA-F]{1,4}:){1,4}(?::[0-9a-fA-F]{1,4}){1,3}|(?:[0-9a-fA-F]{1,4}:){1,3}(?::[0-9a-fA-F]{1,4}){1,4}|(?:[0-9a-fA-F]{1,4}:){1,2}(?::[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:(?::[0-9a-fA-F]{1,4}){1,6}|:(?::[0-9a-fA-F]{1,4}){1,7}|::)(\/\d{1,3})$/;

/**
 * A data network delegates /64s from within its IPv6 pool, so the pool prefix
 * must leave room for them: `isIPv6PoolValid`, internal/api/server/api_data_networks.go.
 */
export const IPV6_POOL_MIN_PREFIX = 48;
export const IPV6_POOL_MAX_PREFIX = 60;

export function prefixLength(value: string): number | null {
  const slash = value.lastIndexOf("/");
  if (slash < 0) return null;

  const digits = value.slice(slash + 1);
  if (!/^\d{1,3}$/.test(digits)) return null;

  return Number(digits);
}

export function getMaxPrefixLength(value: string): number {
  return value.includes(":") ? 128 : 32;
}

export function isValidIpv4Cidr(value: string): boolean {
  if (!cidrRegex.test(value)) return false;

  const len = prefixLength(value);

  return len !== null && len >= 0 && len <= 32;
}

export function isValidIpv6Cidr(value: string): boolean {
  if (!ipv6CidrRegex.test(value)) return false;

  const len = prefixLength(value);

  return len !== null && len >= 0 && len <= 128;
}

export function isValidCidr(value: string): boolean {
  if (!value) return true;

  return isValidIpv4Cidr(value) || isValidIpv6Cidr(value);
}

export function isValidIpv6PoolCidr(value: string): boolean {
  if (!value) return true;
  if (!isValidIpv6Cidr(value)) return false;

  const len = prefixLength(value) as number;

  return len >= IPV6_POOL_MIN_PREFIX && len <= IPV6_POOL_MAX_PREFIX;
}
