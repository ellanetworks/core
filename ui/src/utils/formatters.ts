// ──────────────────────────────────────────────────────
// Shared byte-formatting helpers
// ──────────────────────────────────────────────────────

export type DataUnit = "B" | "KB" | "MB" | "GB" | "TB";

export const UNIT_FACTORS: Record<DataUnit, number> = {
  B: 1,
  KB: 1024,
  MB: 1024 ** 2,
  GB: 1024 ** 3,
  TB: 1024 ** 4,
};

export const chooseUnitFromMax = (maxBytes: number): DataUnit => {
  if (maxBytes >= UNIT_FACTORS.TB) return "TB";
  if (maxBytes >= UNIT_FACTORS.GB) return "GB";
  if (maxBytes >= UNIT_FACTORS.MB) return "MB";
  if (maxBytes >= UNIT_FACTORS.KB) return "KB";
  return "B";
};

export const formatBytesWithUnit = (bytes: number, unit: DataUnit): string => {
  if (!Number.isFinite(bytes)) return "";
  const factor = UNIT_FACTORS[unit];
  const value = bytes / factor;
  const decimals = value >= 100 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(decimals)} ${unit}`;
};

export const formatBytesAutoUnit = (bytes: number): string => {
  if (!Number.isFinite(bytes)) return "";
  const unit = chooseUnitFromMax(Math.abs(bytes));
  return formatBytesWithUnit(bytes, unit);
};

// ──────────────────────────────────────────────────────
// Protocol helpers (for flow reports)
// ──────────────────────────────────────────────────────

export const PROTOCOL_NAMES: Record<number, string> = {
  0: "HOPOPT",
  1: "ICMP",
  2: "IGMP",
  3: "GGP",
  4: "IPv4",
  5: "ST",
  6: "TCP",
  7: "CBT",
  8: "EGP",
  9: "IGP",
  10: "BBN-RCC-MON",
  11: "NVP-II",
  12: "PUP",
  13: "ARGUS",
  14: "EMCON",
  15: "XNET",
  16: "CHAOS",
  17: "UDP",
  18: "MUX",
  19: "DCN-MEAS",
  20: "HMP",
  21: "PRM",
  22: "XNS-IDP",
  23: "TRUNK-1",
  24: "TRUNK-2",
  25: "LEAF-1",
  26: "LEAF-2",
  27: "RDP",
  28: "IRTP",
  29: "ISO-TP4",
  30: "NETBLT",
  31: "MFE-NSP",
  32: "MERIT-INP",
  33: "DCCP",
  34: "3PC",
  35: "IDPR",
  36: "XTP",
  37: "DDP",
  38: "IDPR-CMTP",
  39: "TP++",
  40: "IL",
  41: "IPv6",
  42: "SDRP",
  43: "IPv6-Route",
  44: "IPv6-Frag",
  45: "IDRP",
  46: "RSVP",
  47: "GRE",
  48: "DSR",
  49: "BNA",
  50: "ESP",
  51: "AH",
  52: "I-NLSP",
  53: "SWIPE",
  54: "NARP",
  55: "Min-IPv4",
  56: "TLSP",
  57: "SKIP",
  58: "IPv6-ICMP",
  59: "IPv6-NoNxt",
  60: "IPv6-Opts",
  62: "CFTP",
  64: "SAT-EXPAK",
  65: "KRYPTOLAN",
  66: "RVD",
  67: "IPPC",
  69: "SAT-MON",
  70: "VISA",
  71: "IPCV",
  72: "CPNX",
  73: "CPHB",
  74: "WSN",
  75: "PVP",
  76: "BR-SAT-MON",
  77: "SUN-ND",
  78: "WB-MON",
  79: "WB-EXPAK",
  80: "ISO-IP",
  81: "VMTP",
  82: "SECURE-VMTP",
  83: "VINES",
  84: "IPTM",
  85: "NSFNET-IGP",
  86: "DGP",
  87: "TCF",
  88: "EIGRP",
  89: "OSPFIGP",
  90: "Sprite-RPC",
  91: "LARP",
  92: "MTP",
  93: "AX.25",
  94: "IPIP",
  95: "MICP",
  96: "SCC-SP",
  97: "ETHERIP",
  98: "ENCAP",
  100: "GMTP",
  101: "IFMP",
  102: "PNNI",
  103: "PIM",
  104: "ARIS",
  105: "SCPS",
  106: "QNX",
  107: "A/N",
  108: "IPComp",
  109: "SNP",
  110: "Compaq-Peer",
  111: "IPX-in-IP",
  112: "VRRP",
  113: "PGM",
  115: "L2TP",
  116: "DDX",
  117: "IATP",
  118: "STP",
  119: "SRP",
  120: "UTI",
  121: "SMP",
  122: "SM",
  123: "PTP",
  124: "ISIS",
  125: "FIRE",
  126: "CRTP",
  127: "CRUDP",
  128: "SSCOPMCE",
  129: "IPLT",
  130: "SPS",
  131: "PIPE",
  132: "SCTP",
  133: "FC",
  134: "RSVP-E2E-IGNORE",
  135: "Mobility",
  136: "UDPLite",
  137: "MPLS-in-IP",
  138: "manet",
  139: "HIP",
  140: "Shim6",
  141: "WESP",
  142: "ROHC",
  143: "Ethernet",
  144: "AGGFRAG",
  145: "NSH",
};

export const formatProtocol = (value: number): string =>
  PROTOCOL_NAMES[value] ?? String(value);

/**
 * Stable colors for well-known protocols, matching the PIE_COLORS palette
 * used on the Traffic page so chips look consistent across views.
 */
export const PROTOCOL_CHIP_COLORS: Record<number, string> = {
  6: "#2196F3", // TCP  — blue
  17: "#4CAF50", // UDP  — green
  1: "#FF9800", // ICMP — orange
  58: "#E91E63", // IPv6-ICMP — pink
  47: "#9C27B0", // GRE  — purple
  132: "#00BCD4", // SCTP — cyan
};

// ──────────────────────────────────────────────────────
// Date / time formatting
// ──────────────────────────────────────────────────────

/**
 * Human-readable date-time format used across the UI.
 * Example: "Mar 8, 14:32"   (same day) or "Mar 8, 14:32" (always)
 */
export const formatDateTime = (value: string): string => {
  if (!value) return "";
  const d = new Date(value);
  if (isNaN(d.getTime())) {
    // Strip timezone offset suffix if the string doesn't parse
    return value.replace(/\s*[+-]\d{4}$/, "");
  }
  return d.toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
};

// ──────────────────────────────────────────────────────
// Relative time helper
// ──────────────────────────────────────────────────────

export const formatRelativeTime = (dateString: string): string => {
  const now = Date.now();
  const then = new Date(dateString).getTime();
  const diffMs = now - then;
  if (diffMs < 0) return "just now";

  const seconds = Math.floor(diffMs / 1000);
  if (seconds < 60) return `${seconds}s ago`;

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;

  const days = Math.floor(hours / 24);
  return `${days}d ago`;
};

// ──────────────────────────────────────────────────────
// Shared chart colors
// ──────────────────────────────────────────────────────

export const UPLINK_COLOR = "#FF9800";
export const DOWNLINK_COLOR = "#4254FB";

export const PIE_COLORS = [
  "#2196F3",
  "#4CAF50",
  "#FF9800",
  "#E91E63",
  "#9C27B0",
  "#00BCD4",
  "#FF5722",
  "#795548",
  "#607D8B",
  "#8BC34A",
  "#3F51B5",
  "#CDDC39",
];
