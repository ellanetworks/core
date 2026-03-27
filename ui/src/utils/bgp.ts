import type { BGPImportPrefix } from "@/queries/bgp";

export const ipv4Regex =
  /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;

export const cidrRegex =
  /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}\/\d{1,2}$/;

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
