import React, {
  useMemo,
  useState,
  useEffect,
  useRef,
  useCallback,
} from "react";
import {
  Box,
  Button,
  Typography,
  CircularProgress,
  Alert,
  TextField,
  MenuItem,
  IconButton,
  Tab,
  Tabs,
  Tooltip,
  Chip,
} from "@mui/material";
import { Edit as EditIcon } from "@mui/icons-material";
import DeleteOutlineIcon from "@mui/icons-material/DeleteOutline";
import NorthIcon from "@mui/icons-material/North";
import SouthIcon from "@mui/icons-material/South";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import {
  DataGrid,
  type GridColDef,
  type GridPaginationModel,
  type GridSortModel,
} from "@mui/x-data-grid";
import { BarChart } from "@mui/x-charts/BarChart";
import { PieChart } from "@mui/x-charts/PieChart";
import {
  getUsage,
  getUsageRetentionPolicy,
  clearUsageData,
  type UsageResult,
  type UsageRetentionPolicy,
} from "@/queries/usage";
import {
  listFlowReports,
  clearFlowReports,
  getFlowReportsRetentionPolicy,
  getFlowReportStats,
  type FlowReport,
  type ListFlowReportsResponse,
  type FlowReportsRetentionPolicy,
  type FlowReportStatsResponse,
  type FlowReportFilters,
} from "@/queries/flow_reports";
import {
  getFlowAccountingInfo,
  type FlowAccountingInfo,
} from "@/queries/flow_accounting";
import { useAuth } from "@/contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";
import { Link, useLocation, useNavigate } from "react-router-dom";
import EditUsageRetentionPolicyModal from "@/components/EditUsageRetentionPolicyModal";
import EditFlowReportsRetentionPolicyModal from "@/components/EditFlowReportsRetentionPolicyModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";

const MAX_WIDTH = 1400;

// ──────────────────────────────────────────────────────
// Shared byte-formatting helpers
// ──────────────────────────────────────────────────────

type DataUnit = "B" | "KB" | "MB" | "GB" | "TB";

const UNIT_FACTORS: Record<DataUnit, number> = {
  B: 1,
  KB: 1024,
  MB: 1024 ** 2,
  GB: 1024 ** 3,
  TB: 1024 ** 4,
};

const chooseUnitFromMax = (maxBytes: number): DataUnit => {
  if (maxBytes >= UNIT_FACTORS.TB) return "TB";
  if (maxBytes >= UNIT_FACTORS.GB) return "GB";
  if (maxBytes >= UNIT_FACTORS.MB) return "MB";
  if (maxBytes >= UNIT_FACTORS.KB) return "KB";
  return "B";
};

const formatBytesWithUnit = (bytes: number, unit: DataUnit): string => {
  if (!Number.isFinite(bytes)) return "";
  const factor = UNIT_FACTORS[unit];
  const value = bytes / factor;
  const decimals = value >= 100 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(decimals)} ${unit}`;
};

const formatBytesAutoUnit = (bytes: number): string => {
  if (!Number.isFinite(bytes)) return "";
  const unit = chooseUnitFromMax(Math.abs(bytes));
  return formatBytesWithUnit(bytes, unit);
};

// ──────────────────────────────────────────────────────
// Protocol helpers (for flow reports)
// ──────────────────────────────────────────────────────

const PROTOCOL_NAMES: Record<number, string> = {
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

const PROTOCOL_NUMBER_BY_NAME: Record<string, number> = Object.fromEntries(
  Object.entries(PROTOCOL_NAMES).map(([num, name]) => [
    name.toUpperCase(),
    Number(num),
  ]),
);

const formatProtocol = (value: number): string =>
  PROTOCOL_NAMES[value] ?? String(value);

const resolveProtocolFilter = (input: string): string => {
  const trimmed = input.trim();
  if (trimmed === "") return "";
  if (/^\d+$/.test(trimmed)) return trimmed;
  const num = PROTOCOL_NUMBER_BY_NAME[trimmed.toUpperCase()];
  return num !== undefined ? String(num) : "";
};

// ──────────────────────────────────────────────────────
// Date defaults
// ──────────────────────────────────────────────────────

const getDefaultDateRange = () => {
  const today = new Date();
  const sevenDaysAgo = new Date();
  sevenDaysAgo.setDate(today.getDate() - 6);
  const format = (d: Date) => d.toISOString().slice(0, 10);
  return { startDate: format(sevenDaysAgo), endDate: format(today) };
};

// ──────────────────────────────────────────────────────
// Usage row types
// ──────────────────────────────────────────────────────

type UsageRow = {
  id: string;
  subscriber: string;
  uplink_bytes: number;
  downlink_bytes: number;
  total_bytes: number;
};

type UsagePerDayRow = {
  date: string;
  uplink_bytes: number;
  downlink_bytes: number;
  total_bytes: number;
};

// ──────────────────────────────────────────────────────
// Tab paths
// ──────────────────────────────────────────────────────

const TAB_PATHS = ["/traffic/usage", "/traffic/flows"] as const;

// ──────────────────────────────────────────────────────
// Pie chart color palette
// ──────────────────────────────────────────────────────

const PIE_COLORS = [
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

// ──────────────────────────────────────────────────────
// Main component
// ──────────────────────────────────────────────────────

const Traffic: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin";

  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));

  const location = useLocation();
  const navigate = useNavigate();

  const currentTab = TAB_PATHS.includes(
    location.pathname as (typeof TAB_PATHS)[number],
  )
    ? (location.pathname as (typeof TAB_PATHS)[number])
    : TAB_PATHS[0];

  const handleTabChange = useCallback(
    (_: React.SyntheticEvent, newValue: string) => {
      navigate(newValue);
    },
    [navigate],
  );

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: "#F5F5F5" } },
      }),
    [theme],
  );

  // ── Shared state ────────────────────────────────────
  const [{ startDate, endDate }, setDateRange] = useState(getDefaultDateRange);
  const [selectedSubscriber, setSelectedSubscriber] = useState("");
  const { showSnackbar } = useSnackbar();

  // ── Usage state ─────────────────────────────────────
  const [usagePaginationModel, setUsagePaginationModel] =
    useState<GridPaginationModel>({ page: 0, pageSize: 25 });
  const [isEditUsageRetentionOpen, setEditUsageRetentionOpen] = useState(false);
  const [isUsageClearModalOpen, setUsageClearModalOpen] = useState(false);

  // ── Flow Reports state ──────────────────────────────
  const [flowPaginationModel, setFlowPaginationModel] =
    useState<GridPaginationModel>({ page: 0, pageSize: 25 });
  const [flowSortModel, setFlowSortModel] = useState<GridSortModel>([]);
  const [protocolFilter, setProtocolFilter] = useState("");
  const [sourceIpFilter, setSourceIpFilter] = useState("");
  const [destinationIpFilter, setDestinationIpFilter] = useState("");
  const [appliedProtocol, setAppliedProtocol] = useState("");
  const [appliedSourceIp, setAppliedSourceIp] = useState("");
  const [appliedDestinationIp, setAppliedDestinationIp] = useState("");
  const [directionFilter, setDirectionFilter] = useState("");
  const [isEditFlowRetentionOpen, setEditFlowRetentionOpen] = useState(false);
  const [isFlowClearModalOpen, setFlowClearModalOpen] = useState(false);

  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const scheduleDebounce = (
    setter: React.Dispatch<React.SetStateAction<string>>,
    value: string,
  ) => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setter(value);
      setFlowPaginationModel((prev) => ({ ...prev, page: 0 }));
    }, 400);
  };

  const scheduleProtocolDebounce = (raw: string) => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setAppliedProtocol(resolveProtocolFilter(raw));
      setFlowPaginationModel((prev) => ({ ...prev, page: 0 }));
    }, 400);
  };

  useEffect(() => {
    setFlowPaginationModel((prev) => ({ ...prev, page: 0 }));
  }, [startDate, endDate]);

  // ── Usage queries ───────────────────────────────────

  const { data: usageRetentionPolicy, refetch: refetchUsageRetention } =
    useQuery<UsageRetentionPolicy>({
      queryKey: ["usageRetentionPolicy"],
      enabled: authReady && !!accessToken,
      queryFn: () => getUsageRetentionPolicy(accessToken || ""),
    });

  const {
    data: usagePerSubscriberData,
    isLoading: isUsagePerSubLoading,
    refetch: refetchUsagePerSub,
  } = useQuery<UsageResult>({
    queryKey: ["usagePerSubscriber", startDate, endDate, selectedSubscriber],
    queryFn: () =>
      getUsage(
        accessToken || "",
        startDate,
        endDate,
        selectedSubscriber,
        "subscriber",
      ),
    enabled: !!accessToken && !!startDate && !!endDate,
    placeholderData: (prev) => prev,
  });

  const {
    data: usagePerDayData,
    isLoading: isUsagePerDayLoading,
    refetch: refetchUsagePerDay,
  } = useQuery<UsageResult>({
    queryKey: ["usagePerDay", startDate, endDate, selectedSubscriber],
    queryFn: () =>
      getUsage(
        accessToken || "",
        startDate,
        endDate,
        selectedSubscriber,
        "day",
      ),
    enabled: !!accessToken && !!startDate && !!endDate,
    placeholderData: (prev) => prev,
  });

  // ── Flow Reports queries ────────────────────────────

  const flowPageOneBased = flowPaginationModel.page + 1;

  const activeFlowFilters: FlowReportFilters = useMemo(() => {
    const f: FlowReportFilters = { start: startDate, end: endDate };
    if (selectedSubscriber) f.subscriber_id = selectedSubscriber;
    if (appliedProtocol) f.protocol = appliedProtocol;
    if (appliedSourceIp) f.source_ip = appliedSourceIp;
    if (appliedDestinationIp) f.destination_ip = appliedDestinationIp;
    if (directionFilter) f.direction = directionFilter;
    return f;
  }, [
    startDate,
    endDate,
    selectedSubscriber,
    appliedProtocol,
    appliedSourceIp,
    appliedDestinationIp,
    directionFilter,
  ]);

  const { data: flowRetentionPolicy, refetch: refetchFlowRetention } =
    useQuery<FlowReportsRetentionPolicy>({
      queryKey: ["flowReportsRetentionPolicy"],
      queryFn: () => getFlowReportsRetentionPolicy(accessToken || ""),
      enabled: authReady && !!accessToken,
    });

  const {
    data: flowData,
    isLoading: isFlowLoading,
    refetch: refetchFlowReports,
  } = useQuery<ListFlowReportsResponse>({
    queryKey: [
      "flowReports",
      flowPageOneBased,
      flowPaginationModel.pageSize,
      activeFlowFilters,
    ],
    queryFn: () =>
      listFlowReports(
        accessToken || "",
        flowPageOneBased,
        flowPaginationModel.pageSize,
        activeFlowFilters,
      ),
    enabled: authReady && !!accessToken,
    placeholderData: (prev) => prev,
    refetchInterval: 5000,
  });

  const { data: flowStatsData } = useQuery<FlowReportStatsResponse>({
    queryKey: ["flowReportStats", activeFlowFilters],
    queryFn: () => getFlowReportStats(accessToken || "", activeFlowFilters),
    enabled: authReady && !!accessToken,
    placeholderData: (prev) => prev,
    refetchInterval: 5000,
  });

  const { data: flowAccountingInfo } = useQuery<FlowAccountingInfo>({
    queryKey: ["flow-accounting"],
    queryFn: () => getFlowAccountingInfo(accessToken || ""),
    enabled: authReady && !!accessToken,
  });

  // ── Derived usage data ──────────────────────────────

  const usageRows: UsageRow[] = useMemo(() => {
    if (!usagePerSubscriberData) return [];
    const items: UsageRow[] = [];
    for (const entry of usagePerSubscriberData) {
      const subscriber = Object.keys(entry)[0];
      const usage = entry[subscriber];
      if (!subscriber || !usage) continue;
      items.push({
        id: subscriber,
        subscriber,
        uplink_bytes: usage.uplink_bytes,
        downlink_bytes: usage.downlink_bytes,
        total_bytes: usage.total_bytes,
      });
    }
    items.sort((a, b) => b.total_bytes - a.total_bytes);
    return items;
  }, [usagePerSubscriberData]);

  const subscriberOptions = useMemo(
    () => usageRows.map((r) => r.subscriber),
    [usageRows],
  );

  const dailyRows: UsagePerDayRow[] = useMemo(() => {
    if (!usagePerDayData) return [];
    const items: UsagePerDayRow[] = [];
    for (const entry of usagePerDayData) {
      const date = Object.keys(entry)[0];
      const usage = entry[date];
      if (!date || !usage) continue;
      items.push({
        date,
        uplink_bytes: usage.uplink_bytes,
        downlink_bytes: usage.downlink_bytes,
        total_bytes: usage.total_bytes,
      });
    }
    items.sort((a, b) => a.date.localeCompare(b.date));
    return items;
  }, [usagePerDayData]);

  const maxBytesAcrossData = useMemo(() => {
    let max = 0;
    for (const row of usageRows) {
      if (row.uplink_bytes > max) max = row.uplink_bytes;
      if (row.downlink_bytes > max) max = row.downlink_bytes;
      if (row.total_bytes > max) max = row.total_bytes;
    }
    for (const row of dailyRows) {
      if (row.uplink_bytes > max) max = row.uplink_bytes;
      if (row.downlink_bytes > max) max = row.downlink_bytes;
      if (row.total_bytes > max) max = row.total_bytes;
    }
    return max;
  }, [usageRows, dailyRows]);

  const unit: DataUnit = useMemo(
    () => chooseUnitFromMax(maxBytesAcrossData),
    [maxBytesAcrossData],
  );

  const chartDataset = useMemo(
    () =>
      dailyRows.map((row) => {
        const factor = UNIT_FACTORS[unit];
        return {
          date: row.date,
          downlink: row.downlink_bytes / factor,
          uplink: row.uplink_bytes / factor,
        };
      }),
    [dailyRows, unit],
  );

  const usageColumns: GridColDef<UsageRow>[] = useMemo(
    () => [
      { field: "subscriber", headerName: "Subscriber", flex: 1, minWidth: 200 },
      {
        field: "downlink_bytes",
        headerName: "Downlink (bytes)",
        flex: 1,
        minWidth: 180,
        type: "number",
        valueFormatter: (value: any) =>
          value == null ? "" : formatBytesAutoUnit(Number(value)),
      },
      {
        field: "uplink_bytes",
        headerName: "Uplink (bytes)",
        flex: 1,
        minWidth: 180,
        type: "number",
        valueFormatter: (value: any) =>
          value == null ? "" : formatBytesAutoUnit(Number(value)),
      },
      {
        field: "total_bytes",
        headerName: "Total (bytes)",
        flex: 1,
        minWidth: 180,
        type: "number",
        valueFormatter: (value: any) =>
          value == null ? "" : formatBytesAutoUnit(Number(value)),
      },
    ],
    [],
  );

  // ── Derived flow data ───────────────────────────────

  const flowRows: FlowReport[] = flowData?.items ?? [];
  const flowRowCount = flowData?.total_count ?? 0;

  const protocolColorMap = useMemo(() => {
    const map = new Map<number, string>();
    if (flowStatsData?.protocols?.length) {
      flowStatsData.protocols.forEach((p, i) => {
        map.set(p.protocol, PIE_COLORS[i % PIE_COLORS.length]);
      });
    }
    return map;
  }, [flowStatsData]);

  const flowColumns: GridColDef<FlowReport>[] = useMemo(
    () => [
      {
        field: "subscriber_id",
        headerName: "Subscriber",
        flex: 1,
        minWidth: 160,
      },
      {
        field: "direction",
        headerName: "Direction",
        width: 100,
        sortable: false,
        renderCell: (params) => {
          const dir = params.value as string;
          if (!dir) return null;
          const Icon = dir === "uplink" ? NorthIcon : SouthIcon;
          const title = dir === "uplink" ? "Uplink" : "Downlink";
          const color = dir === "uplink" ? "#FF9800" : "#4254FB";
          return (
            <Tooltip title={title}>
              <Box
                sx={{
                  width: "100%",
                  height: "100%",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  lineHeight: 0,
                  "& svg": { display: "block" },
                }}
              >
                <Icon fontSize="small" sx={{ color }} aria-label={title} />
              </Box>
            </Tooltip>
          );
        },
      },
      {
        field: "source_ip",
        headerName: "Source",
        flex: 1,
        minWidth: 160,
        renderCell: (params) => {
          const row = params.row as FlowReport;
          const proto = row.protocol;
          if (proto === 6 || proto === 17) {
            return `${row.source_ip}:${row.source_port}`;
          }
          return row.source_ip;
        },
      },
      {
        field: "destination_ip",
        headerName: "Destination",
        flex: 1,
        minWidth: 160,
        renderCell: (params) => {
          const row = params.row as FlowReport;
          const proto = row.protocol;
          if (proto === 6 || proto === 17) {
            return `${row.destination_ip}:${row.destination_port}`;
          }
          return row.destination_ip;
        },
      },
      {
        field: "protocol",
        headerName: "Protocol",
        width: 110,
        renderCell: (params) => {
          const value = params.value as number;
          if (value == null) return null;
          const label = formatProtocol(value);
          const bg = protocolColorMap.get(value);
          if (!bg) return label;
          return (
            <Box
              sx={{
                width: "100%",
                height: "100%",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              <Chip
                label={label}
                size="small"
                sx={{
                  backgroundColor: bg,
                  color: "#fff",
                  fontWeight: 600,
                  fontSize: "0.75rem",
                  height: 22,
                }}
              />
            </Box>
          );
        },
      },
      {
        field: "packets",
        headerName: "Packets",
        type: "number",
        width: 110,
        valueFormatter: (value: number) =>
          value == null ? "" : value.toLocaleString(),
      },
      {
        field: "bytes",
        headerName: "Bytes",
        type: "number",
        width: 120,
        valueFormatter: (value: number) =>
          value == null ? "" : formatBytesAutoUnit(value),
      },
      {
        field: "start_time",
        headerName: "Start",
        flex: 1,
        minWidth: 180,
        valueFormatter: (value: string) =>
          value ? new Date(value).toLocaleString() : "",
      },
      {
        field: "end_time",
        headerName: "End",
        flex: 1,
        minWidth: 180,
        valueFormatter: (value: string) =>
          value ? new Date(value).toLocaleString() : "",
      },
    ],
    [theme, protocolColorMap],
  );

  // ── Protocol distribution (donut chart) ─────────────

  const protocolPieData = useMemo(() => {
    if (!flowStatsData?.protocols?.length) return [];
    return flowStatsData.protocols.map((p, i) => ({
      id: p.protocol,
      value: p.count,
      label: formatProtocol(p.protocol),
      color: PIE_COLORS[i % PIE_COLORS.length],
    }));
  }, [flowStatsData]);

  // ── Top 10 destinations uplink (donut chart) ───────────────

  const topDestinationsPieData = useMemo(() => {
    if (!flowStatsData?.top_destinations_uplink?.length) return [];
    return flowStatsData.top_destinations_uplink.map((d, i) => ({
      id: i,
      value: d.count,
      label: d.ip,
      color: PIE_COLORS[i % PIE_COLORS.length],
    }));
  }, [flowStatsData]);

  // ── Handlers ────────────────────────────────────────

  const handleStartChange = (e: React.ChangeEvent<HTMLInputElement>) =>
    setDateRange((prev) => ({ ...prev, startDate: e.target.value }));

  const handleEndChange = (e: React.ChangeEvent<HTMLInputElement>) =>
    setDateRange((prev) => ({ ...prev, endDate: e.target.value }));

  const handleConfirmClearUsage = async () => {
    setUsageClearModalOpen(false);
    if (!accessToken) return;
    try {
      await clearUsageData(accessToken);
      await Promise.allSettled([refetchUsagePerSub(), refetchUsagePerDay()]);
      showSnackbar("All usage data cleared successfully.", "success");
    } catch (error: unknown) {
      showSnackbar(
        `Failed to clear usage data: ${error instanceof Error ? error.message : String(error)}`,
        "error",
      );
    }
  };

  const handleConfirmClearFlows = async () => {
    setFlowClearModalOpen(false);
    if (!accessToken) return;
    try {
      await clearFlowReports(accessToken);
      await refetchFlowReports();
      showSnackbar("All flow report data cleared successfully.", "success");
    } catch (error: unknown) {
      showSnackbar(
        `Failed to clear flow report data: ${error instanceof Error ? error.message : String(error)}`,
        "error",
      );
    }
  };

  // ── Loading ─────────────────────────────────────────

  const isInitialLoading =
    (isUsagePerSubLoading && !usagePerSubscriberData) ||
    (isUsagePerDayLoading && !usagePerDayData);

  // ── DataGrid shared styles ──────────────────────────

  const gridSx = {
    width: "100%",
    border: 1,
    borderColor: "divider",
    "& .MuiDataGrid-cell": {
      borderBottom: "1px solid",
      borderColor: "divider",
    },
    "& .MuiDataGrid-columnHeaders": {
      borderBottom: "1px solid",
      borderColor: "divider",
      backgroundColor: "#F5F5F5",
    },
    "& .MuiDataGrid-footerContainer": {
      borderTop: "1px solid",
      borderColor: "divider",
    },
    "& .MuiDataGrid-columnHeaderTitle": { fontWeight: "bold" },
  };

  // ── Render ──────────────────────────────────────────

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        pt: 6,
        pb: 4,
      }}
    >
      {isInitialLoading ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : (
        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            px: { xs: 2, sm: 4 },
            display: "flex",
            flexDirection: "column",
            gap: 2,
          }}
        >
          {/* Header */}
          <Typography variant="h4">Traffic</Typography>
          <Typography variant="body1" color="text.secondary">
            Monitor network traffic — view aggregated data usage and individual
            flow records collected by the user plane.
          </Typography>

          {/* Shared filters */}
          <Box
            sx={{
              display: "flex",
              flexDirection: { xs: "column", sm: "row" },
              gap: 2,
              alignItems: { xs: "flex-start", sm: "center" },
            }}
          >
            <TextField
              label="Start date"
              type="date"
              value={startDate}
              onChange={handleStartChange}
              InputLabelProps={{ shrink: true }}
              size="small"
            />
            <TextField
              label="End date"
              type="date"
              value={endDate}
              onChange={handleEndChange}
              InputLabelProps={{ shrink: true }}
              size="small"
            />
            <TextField
              select
              label="Subscriber"
              value={selectedSubscriber}
              onChange={(e) => setSelectedSubscriber(e.target.value)}
              size="small"
              sx={{ minWidth: 200 }}
            >
              <MenuItem value="">All subscribers</MenuItem>
              {subscriberOptions.map((sub) => (
                <MenuItem key={sub} value={sub}>
                  {sub}
                </MenuItem>
              ))}
            </TextField>
          </Box>

          {/* Tabs */}
          <Tabs
            value={currentTab}
            onChange={handleTabChange}
            sx={{ borderBottom: 1, borderColor: "divider" }}
          >
            <Tab label="Usage" value={TAB_PATHS[0]} />
            <Tab label="Flows" value={TAB_PATHS[1]} />
          </Tabs>

          {/* ─── Usage tab ────────────────────────────── */}
          {currentTab === TAB_PATHS[0] && (
            <Box
              sx={{ display: "flex", flexDirection: "column", gap: 3, mt: 1 }}
            >
              <Box
                sx={{
                  display: "flex",
                  flexDirection: { xs: "column", sm: "row" },
                  alignItems: { xs: "flex-start", sm: "center" },
                  justifyContent: "space-between",
                  gap: 1,
                }}
              >
                <Box />
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1,
                  }}
                >
                  {canEdit && (
                    <Button
                      variant="outlined"
                      color="error"
                      size="small"
                      startIcon={<DeleteOutlineIcon />}
                      onClick={() => setUsageClearModalOpen(true)}
                      sx={{ flexShrink: 0 }}
                    >
                      Clear All
                    </Button>
                  )}
                  <Typography variant="body2" color="text.secondary">
                    Retention:{" "}
                    <strong>{usageRetentionPolicy?.days ?? "…"}</strong> days
                  </Typography>
                  {canEdit && (
                    <IconButton
                      aria-label="edit usage retention"
                      size="small"
                      onClick={() => setEditUsageRetentionOpen(true)}
                    >
                      <EditIcon fontSize="small" />
                    </IconButton>
                  )}
                </Box>
              </Box>

              {/* Chart */}
              <Box>
                <Typography variant="h6" sx={{ mb: 2 }}>
                  Daily data usage ({selectedSubscriber || "all subscribers"})
                  in {unit}
                </Typography>
                <BarChart
                  dataset={chartDataset}
                  xAxis={[{ scaleType: "band", dataKey: "date" }]}
                  yAxis={[{ label: `Usage (${unit})` }]}
                  series={[
                    { dataKey: "downlink", label: `Downlink (${unit})` },
                    { dataKey: "uplink", label: `Uplink (${unit})` },
                  ]}
                  height={300}
                />
              </Box>

              {/* Usage table */}
              <ThemeProvider theme={gridTheme}>
                <DataGrid<UsageRow>
                  rows={usageRows}
                  columns={usageColumns}
                  getRowId={(row) => row.id}
                  paginationModel={usagePaginationModel}
                  onPaginationModelChange={setUsagePaginationModel}
                  pageSizeOptions={[10, 25, 50, 100]}
                  columnVisibilityModel={{ subscriber: !isSmDown }}
                  sx={gridSx}
                />
              </ThemeProvider>
            </Box>
          )}

          {/* ─── Flows tab ────────────────────────────── */}
          {currentTab === TAB_PATHS[1] && (
            <Box
              sx={{ display: "flex", flexDirection: "column", gap: 3, mt: 1 }}
            >
              {flowAccountingInfo?.enabled === false && (
                <Alert severity="warning">
                  Flow accounting is disabled. Flows are not being collected and
                  will not appear on this page. You can enable it in the{" "}
                  <Link to="/networking?tab=flow-accounting">
                    networking settings
                  </Link>
                  .
                </Alert>
              )}
              <Box
                sx={{
                  display: "flex",
                  flexDirection: { xs: "column", sm: "row" },
                  alignItems: { xs: "flex-start", sm: "center" },
                  justifyContent: "space-between",
                  gap: 1,
                }}
              >
                <Box />
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1,
                  }}
                >
                  {canEdit && (
                    <Button
                      variant="outlined"
                      color="error"
                      size="small"
                      startIcon={<DeleteOutlineIcon />}
                      onClick={() => setFlowClearModalOpen(true)}
                      sx={{ flexShrink: 0 }}
                    >
                      Clear All
                    </Button>
                  )}
                  <Typography variant="body2" color="text.secondary">
                    Retention:{" "}
                    <strong>{flowRetentionPolicy?.days ?? "…"}</strong> days
                  </Typography>
                  {canEdit && (
                    <IconButton
                      aria-label="edit flow reports retention"
                      size="small"
                      onClick={() => setEditFlowRetentionOpen(true)}
                    >
                      <EditIcon fontSize="small" />
                    </IconButton>
                  )}
                </Box>
              </Box>

              {/* Donut charts row */}
              {(protocolPieData.length > 0 ||
                topDestinationsPieData.length > 0) && (
                <Box
                  sx={{
                    display: "grid",
                    gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
                    gap: 3,
                    alignItems: "start",
                  }}
                >
                  {protocolPieData.length > 0 && (
                    <Box>
                      <Typography variant="h6" sx={{ mb: 1 }}>
                        Protocols
                      </Typography>
                      <PieChart
                        series={[
                          {
                            data: protocolPieData,
                            innerRadius: 30,
                            outerRadius: 80,
                            paddingAngle: 2,
                            cornerRadius: 5,
                          },
                        ]}
                        height={300}
                        slotProps={{
                          legend: {
                            direction: "horizontal",
                            position: {
                              vertical: "bottom",
                              horizontal: "center",
                            },
                          },
                        }}
                      />
                    </Box>
                  )}
                  {topDestinationsPieData.length > 0 && (
                    <Box>
                      <Typography variant="h6" sx={{ mb: 1 }}>
                        Top 10 Destinations (uplink)
                      </Typography>
                      <PieChart
                        series={[
                          {
                            data: topDestinationsPieData,
                            innerRadius: 30,
                            outerRadius: 80,
                            paddingAngle: 2,
                            cornerRadius: 5,
                          },
                        ]}
                        height={300}
                        slotProps={{
                          legend: {
                            direction: "horizontal",
                            position: {
                              vertical: "bottom",
                              horizontal: "center",
                            },
                          },
                        }}
                      />
                    </Box>
                  )}
                </Box>
              )}

              {/* Flow-specific filters */}
              <Box
                sx={{
                  display: "flex",
                  flexDirection: { xs: "column", sm: "row" },
                  gap: 2,
                  alignItems: { xs: "flex-start", sm: "center" },
                  flexWrap: "wrap",
                }}
              >
                <TextField
                  select
                  label="Direction"
                  value={directionFilter}
                  onChange={(e) => {
                    setDirectionFilter(e.target.value);
                    setFlowPaginationModel((prev) => ({ ...prev, page: 0 }));
                  }}
                  size="small"
                  sx={{ minWidth: 140 }}
                >
                  <MenuItem value="">All</MenuItem>
                  <MenuItem value="uplink">Uplink</MenuItem>
                  <MenuItem value="downlink">Downlink</MenuItem>
                </TextField>
                <TextField
                  label="Protocol"
                  value={protocolFilter}
                  onChange={(e) => {
                    setProtocolFilter(e.target.value);
                    scheduleProtocolDebounce(e.target.value);
                  }}
                  size="small"
                  sx={{ minWidth: 100 }}
                  placeholder="e.g. TCP or 6"
                />
                <TextField
                  label="Source IP"
                  value={sourceIpFilter}
                  onChange={(e) => {
                    setSourceIpFilter(e.target.value);
                    scheduleDebounce(setAppliedSourceIp, e.target.value);
                  }}
                  size="small"
                  sx={{ minWidth: 140 }}
                />
                <TextField
                  label="Destination IP"
                  value={destinationIpFilter}
                  onChange={(e) => {
                    setDestinationIpFilter(e.target.value);
                    scheduleDebounce(setAppliedDestinationIp, e.target.value);
                  }}
                  size="small"
                  sx={{ minWidth: 140 }}
                />
              </Box>

              {/* Flow table */}
              {flowRowCount === 0 && !isFlowLoading ? (
                <EmptyState
                  primaryText="No flow reports found"
                  secondaryText="No flows match the current filters, or flow accounting has not recorded any data yet."
                  button={false}
                  buttonText=""
                  onCreate={() => {}}
                />
              ) : (
                <ThemeProvider theme={gridTheme}>
                  <DataGrid<FlowReport>
                    rows={flowRows}
                    columns={flowColumns}
                    getRowId={(row) => row.id}
                    paginationMode="server"
                    rowCount={flowRowCount}
                    paginationModel={flowPaginationModel}
                    onPaginationModelChange={setFlowPaginationModel}
                    sortModel={flowSortModel}
                    onSortModelChange={setFlowSortModel}
                    sortingMode="client"
                    disableColumnMenu
                    disableRowSelectionOnClick
                    pageSizeOptions={[10, 25, 50, 100]}
                    density="compact"
                    columnVisibilityModel={{}}
                    sx={gridSx}
                  />
                </ThemeProvider>
              )}
            </Box>
          )}
        </Box>
      )}

      {/* ── Modals ───────────────────────────────────── */}
      <EditUsageRetentionPolicyModal
        open={isEditUsageRetentionOpen}
        onClose={() => setEditUsageRetentionOpen(false)}
        onSuccess={() => {
          refetchUsageRetention();
          showSnackbar(
            "Usage retention policy updated successfully.",
            "success",
          );
        }}
        initialData={usageRetentionPolicy || { days: 30 }}
      />
      <EditFlowReportsRetentionPolicyModal
        open={isEditFlowRetentionOpen}
        onClose={() => setEditFlowRetentionOpen(false)}
        onSuccess={() => {
          refetchFlowRetention();
          showSnackbar(
            "Flow reports retention policy updated successfully.",
            "success",
          );
        }}
        initialData={flowRetentionPolicy || { days: 30 }}
      />
      <DeleteConfirmationModal
        title="Clear All Usage Data"
        description="Are you sure you want to clear all usage data? This action cannot be undone."
        open={isUsageClearModalOpen}
        onClose={() => setUsageClearModalOpen(false)}
        onConfirm={handleConfirmClearUsage}
      />
      <DeleteConfirmationModal
        title="Clear All Flow Report Data"
        description="Are you sure you want to clear all flow report data? This action cannot be undone."
        open={isFlowClearModalOpen}
        onClose={() => setFlowClearModalOpen(false)}
        onConfirm={handleConfirmClearFlows}
      />
    </Box>
  );
};

export default Traffic;
