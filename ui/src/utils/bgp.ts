import type { BGPImportPrefix } from "@/queries/bgp";

export const ipv4Regex =
  /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;

export const cidrRegex =
  /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}\/\d{1,2}$/;

export const ipRegex =
  /^((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}|(([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,7}:|([0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|([0-9a-fA-F]{1,4}:){1,5}(:[0-9a-fA-F]{1,4}){1,2}|([0-9a-fA-F]{1,4}:){1,4}(:[0-9a-fA-F]{1,4}){1,3}|([0-9a-fA-F]{1,4}:){1,3}(:[0-9a-fA-F]{1,4}){1,4}|([0-9a-fA-F]{1,4}:){1,2}(:[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:((:[0-9a-fA-F]{1,4}){1,6})|:((:[0-9a-fA-F]{1,4}){1,7}|:)|fe80:(:[0-9a-fA-F]{0,4}){0,4}%[0-9a-zA-Z]+|::(ffff(:0{1,4})?:)?(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}|([0-9a-fA-F]{1,4}:){1,4}:(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))$/;

export type ImportPreset = "none" | "default-route" | "all" | "custom";

export function detectPreset(prefixes: BGPImportPrefix[]): ImportPreset {
  if (prefixes.length === 0) return "none";
  if (
    prefixes.length === 1 &&
    prefixes[0].prefix === "0.0.0.0/0" &&
    prefixes[0].maxLength === 0
  )
    return "default-route";
  if (
    prefixes.length === 1 &&
    prefixes[0].prefix === "0.0.0.0/0" &&
    prefixes[0].maxLength === 32
  )
    return "all";
  return "custom";
}

export function getImportPolicyLabel(
  prefixes: BGPImportPrefix[] | undefined,
): string {
  if (!prefixes || prefixes.length === 0) return "Deny All";
  if (
    prefixes.length === 1 &&
    prefixes[0].prefix === "0.0.0.0/0" &&
    prefixes[0].maxLength === 0
  )
    return "Default Route Only";
  if (
    prefixes.length === 1 &&
    prefixes[0].prefix === "0.0.0.0/0" &&
    prefixes[0].maxLength === 32
  )
    return "Accept All";
  return "Custom";
}
