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
import {
  Link,
  useLocation,
  useNavigate,
  useSearchParams,
} from "react-router-dom";
import EditUsageRetentionPolicyModal from "@/components/EditUsageRetentionPolicyModal";
import EditFlowReportsRetentionPolicyModal from "@/components/EditFlowReportsRetentionPolicyModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import {
  type DataUnit,
  UNIT_FACTORS,
  chooseUnitFromMax,
  formatBytesWithUnit,
  formatBytesAutoUnit,
  formatProtocol,
  formatDateTime,
  PROTOCOL_NAMES,
  UPLINK_COLOR,
  DOWNLINK_COLOR,
  PIE_COLORS,
} from "@/utils/formatters";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

/** Shared cell renderer for subscriber IMSI links in data grids. */
const renderSubscriberLink = (params: { value?: unknown }) => {
  const imsi = params.value as string;
  if (!imsi) return null;
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        width: "100%",
        height: "100%",
      }}
    >
      <Link
        to={`/subscribers/${imsi}`}
        style={{ textDecoration: "none" }}
        onClick={(e: React.MouseEvent) => e.stopPropagation()}
      >
        <Typography
          variant="body2"
          sx={{
            fontFamily: "monospace",
            color: (t) => t.palette.link,
            textDecoration: "underline",
            "&:hover": { textDecoration: "underline" },
          }}
        >
          {imsi}
        </Typography>
      </Link>
    </Box>
  );
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
  const [searchParams] = useSearchParams();

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
        palette: { DataGrid: { headerBg: theme.palette.backgroundSubtle } },
      }),
    [theme],
  );

  // ── Shared state ────────────────────────────────────
  const [{ startDate, endDate }, setDateRange] = useState(getDefaultDateRange);
  const [selectedSubscriber, setSelectedSubscriber] = useState(
    () => searchParams.get("subscriber_id") || "",
  );
  const { showSnackbar } = useSnackbar();

  // ── Usage state ─────────────────────────────────────
  const [usagePaginationModel, setUsagePaginationModel] =
    useState<GridPaginationModel>({ page: 0, pageSize: 25 });
  const [isEditUsageRetentionOpen, setEditUsageRetentionOpen] = useState(false);
  const [isUsageClearModalOpen, setUsageClearModalOpen] = useState(false);

  // ── Flow Reports state ──────────────────────────────
  const [flowPaginationModel, setFlowPaginationModel] =
    useState<GridPaginationModel>({ page: 0, pageSize: 25 });
  const [sourceFilter, setSourceFilter] = useState("");
  const [destinationFilter, setDestinationFilter] = useState("");
  const [appliedProtocol, setAppliedProtocol] = useState("");
  const [appliedSource, setAppliedSource] = useState("");
  const [appliedDestination, setAppliedDestination] = useState("");
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
    if (appliedSource) f.source = appliedSource;
    if (appliedDestination) f.destination = appliedDestination;
    if (directionFilter) f.direction = directionFilter;
    return f;
  }, [
    startDate,
    endDate,
    selectedSubscriber,
    appliedProtocol,
    appliedSource,
    appliedDestination,
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

  // Separate query for available protocol options (unfiltered by protocol).
  // Only fires when a protocol filter is active; otherwise reuses flowStatsData.
  const filtersWithoutProtocol: FlowReportFilters = useMemo(() => {
    const { protocol: _ignored, ...rest } = activeFlowFilters;
    return rest;
  }, [activeFlowFilters]);

  const { data: protocolOptionsRaw } = useQuery<FlowReportStatsResponse>({
    queryKey: ["flowReportProtocolOptions", filtersWithoutProtocol],
    queryFn: () =>
      getFlowReportStats(accessToken || "", filtersWithoutProtocol),
    enabled: authReady && !!accessToken,
    placeholderData: (prev) => prev,
    refetchInterval: 5000,
  });

  const protocolOptionsData = protocolOptionsRaw ?? flowStatsData;

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
      {
        field: "subscriber",
        headerName: "Subscriber",
        flex: 1,
        minWidth: 140,
        renderCell: renderSubscriberLink,
      },
      {
        field: "downlink_bytes",
        headerName: "Downlink (bytes)",
        flex: 1,
        minWidth: 120,
        type: "number",
        valueFormatter: (value: any) =>
          value == null ? "" : formatBytesAutoUnit(Number(value)),
      },
      {
        field: "uplink_bytes",
        headerName: "Uplink (bytes)",
        flex: 1,
        minWidth: 120,
        type: "number",
        valueFormatter: (value: any) =>
          value == null ? "" : formatBytesAutoUnit(Number(value)),
      },
      {
        field: "total_bytes",
        headerName: "Total (bytes)",
        flex: 1,
        minWidth: 120,
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
    if (protocolOptionsData?.protocols?.length) {
      protocolOptionsData.protocols.forEach((p, i) => {
        map.set(p.protocol, PIE_COLORS[i % PIE_COLORS.length]);
      });
    }
    return map;
  }, [protocolOptionsData]);

  const flowColumns: GridColDef<FlowReport>[] = useMemo(
    () => [
      {
        field: "subscriber_id",
        headerName: "Subscriber",
        flex: 1,
        minWidth: 120,
        renderCell: renderSubscriberLink,
      },
      {
        field: "direction",
        headerName: "Direction",
        width: 110,
        sortable: false,
        renderCell: (params) => {
          const dir = params.value as string;
          if (!dir) return null;
          const Icon = dir === "uplink" ? NorthIcon : SouthIcon;
          const title = dir === "uplink" ? "Uplink" : "Downlink";
          const color = dir === "uplink" ? UPLINK_COLOR : DOWNLINK_COLOR;
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
        minWidth: 100,
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
        minWidth: 100,
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
        flex: 0.5,
        minWidth: 80,
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
        flex: 0.5,
        minWidth: 80,
        valueFormatter: (value: number) =>
          value == null ? "" : value.toLocaleString(),
      },
      {
        field: "bytes",
        headerName: "Bytes",
        type: "number",
        flex: 0.5,
        minWidth: 80,
        valueFormatter: (value: number) =>
          value == null ? "" : formatBytesAutoUnit(value),
      },
      {
        field: "start_time",
        headerName: "Start",
        flex: 0.8,
        minWidth: 120,
        valueFormatter: (value: string) => (value ? formatDateTime(value) : ""),
      },
      {
        field: "end_time",
        headerName: "End",
        flex: 0.8,
        minWidth: 120,
        valueFormatter: (value: string) => (value ? formatDateTime(value) : ""),
      },
    ],
    [theme, protocolColorMap],
  );

  // ── Protocol distribution (donut chart) ─────────────

  const protocolPieData = useMemo(() => {
    if (!flowStatsData?.protocols?.length) return [];
    return flowStatsData.protocols.map((p) => ({
      id: p.protocol,
      value: p.count,
      label: formatProtocol(p.protocol),
      color: protocolColorMap.get(p.protocol) ?? PIE_COLORS[0],
    }));
  }, [flowStatsData, protocolColorMap]);

  // ── Top 10 destinations uplink (donut chart) ───────────────

  const destinationColorRef = useRef(new Map<string, string>());

  // Reset color map when the time range changes to avoid unbounded growth
  useEffect(() => {
    destinationColorRef.current.clear();
  }, [startDate, endDate]);

  const topDestinationsPieData = useMemo(() => {
    if (!flowStatsData?.top_destinations_uplink?.length) return [];
    const colorMap = destinationColorRef.current;
    return flowStatsData.top_destinations_uplink.map((d, i) => {
      if (!colorMap.has(d.ip)) {
        colorMap.set(d.ip, PIE_COLORS[colorMap.size % PIE_COLORS.length]);
      }
      return {
        id: i,
        value: d.count,
        label: d.ip,
        color: colorMap.get(d.ip)!,
      };
    });
  }, [flowStatsData]);

  // ── Pie chart click handlers ─────────────────────────

  const handleProtocolPieClick = useCallback(
    (dataIndex: number) => {
      const clicked = protocolPieData[dataIndex];
      if (clicked) {
        const value = String(clicked.id);
        setAppliedProtocol((prev) => (prev === value ? "" : value));
        setFlowPaginationModel((prev) => ({ ...prev, page: 0 }));
      }
    },
    [protocolPieData],
  );

  const handleDestinationPieClick = useCallback(
    (dataIndex: number) => {
      const clicked = topDestinationsPieData[dataIndex];
      if (clicked) {
        const isActive =
          directionFilter === "uplink" && appliedDestination === clicked.label;
        if (isActive) {
          setDirectionFilter("");
          setDestinationFilter("");
          setAppliedDestination("");
        } else {
          setDirectionFilter("uplink");
          setDestinationFilter(clicked.label);
          setAppliedDestination(clicked.label);
        }
        setFlowPaginationModel((prev) => ({ ...prev, page: 0 }));
      }
    },
    [topDestinationsPieData, directionFilter, appliedDestination],
  );

  // ── Handlers ────────────────────────────────────────

  const handleStartChange = (e: React.ChangeEvent<HTMLInputElement>) =>
    setDateRange((prev) => ({ ...prev, startDate: e.target.value }));

  const handleEndChange = (e: React.ChangeEvent<HTMLInputElement>) =>
    setDateRange((prev) => ({ ...prev, endDate: e.target.value }));

  const handleConfirmClearUsage = async () => {
    if (!accessToken) return;
    try {
      await clearUsageData(accessToken);
      await Promise.allSettled([refetchUsagePerSub(), refetchUsagePerDay()]);
      setUsageClearModalOpen(false);
      showSnackbar("All usage data cleared successfully.", "success");
    } catch (error: unknown) {
      setUsageClearModalOpen(false);
      showSnackbar(
        `Failed to clear usage data: ${error instanceof Error ? error.message : String(error)}`,
        "error",
      );
    }
  };

  const handleConfirmClearFlows = async () => {
    if (!accessToken) return;
    try {
      await clearFlowReports(accessToken);
      await refetchFlowReports();
      setFlowClearModalOpen(false);
      showSnackbar("All flow report data cleared successfully.", "success");
    } catch (error: unknown) {
      setFlowClearModalOpen(false);
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
      backgroundColor: "backgroundSubtle",
    },
    "& .MuiDataGrid-footerContainer": {
      borderTop: "1px solid",
      borderColor: "divider",
    },
  };

  // ── Render ──────────────────────────────────────────

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      {isInitialLoading ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : (
        <Box
          sx={{
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
                      color="primary"
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
                    {
                      dataKey: "downlink",
                      label: `Downlink (${unit})`,
                      color: DOWNLINK_COLOR,
                    },
                    {
                      dataKey: "uplink",
                      label: `Uplink (${unit})`,
                      color: UPLINK_COLOR,
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

              {/* Usage table */}
              <ThemeProvider theme={gridTheme}>
                <DataGrid<UsageRow>
                  rows={usageRows}
                  columns={usageColumns}
                  getRowId={(row) => row.id}
                  paginationModel={usagePaginationModel}
                  onPaginationModelChange={setUsagePaginationModel}
                  pageSizeOptions={[10, 25, 50, 100]}
                  disableColumnMenu
                  disableRowSelectionOnClick
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
                  <Link to="/networking/flow-accounting">
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
                      color="primary"
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
                            valueFormatter: (item) => {
                              const total = protocolPieData.reduce(
                                (s, d) => s + d.value,
                                0,
                              );
                              return total > 0
                                ? `${((item.value / total) * 100).toFixed(1)}%`
                                : "0%";
                            },
                          },
                        ]}
                        height={300}
                        sx={{
                          "& .MuiPieArc-root": {
                            transitionProperty: "opacity, filter",
                          },
                        }}
                        onItemClick={(_event, d) =>
                          handleProtocolPieClick(d.dataIndex)
                        }
                        slotProps={{
                          legend: {
                            direction: "horizontal",
                            position: {
                              vertical: "bottom",
                              horizontal: "center",
                            },
                            onItemClick: (
                              _event: React.MouseEvent,
                              legendItem: { dataIndex?: number },
                            ) =>
                              handleProtocolPieClick(
                                legendItem.dataIndex ?? -1,
                              ),
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
                            valueFormatter: (item) => {
                              const total = topDestinationsPieData.reduce(
                                (s, d) => s + d.value,
                                0,
                              );
                              return total > 0
                                ? `${((item.value / total) * 100).toFixed(1)}%`
                                : "0%";
                            },
                          },
                        ]}
                        height={300}
                        sx={{
                          "& .MuiPieArc-root": {
                            transitionProperty: "opacity, filter",
                          },
                        }}
                        onItemClick={(_event, d) =>
                          handleDestinationPieClick(d.dataIndex)
                        }
                        slotProps={{
                          legend: {
                            direction: "horizontal",
                            position: {
                              vertical: "bottom",
                              horizontal: "center",
                            },
                            onItemClick: (
                              _event: React.MouseEvent,
                              legendItem: { dataIndex?: number },
                            ) =>
                              handleDestinationPieClick(
                                legendItem.dataIndex ?? -1,
                              ),
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
                  select
                  label="Protocol"
                  value={appliedProtocol}
                  onChange={(e) => {
                    setAppliedProtocol(e.target.value);
                    setFlowPaginationModel((prev) => ({ ...prev, page: 0 }));
                  }}
                  size="small"
                  sx={{ minWidth: 140 }}
                >
                  <MenuItem value="">All</MenuItem>
                  {(protocolOptionsData?.protocols ?? []).map((p) => (
                    <MenuItem key={p.protocol} value={String(p.protocol)}>
                      {formatProtocol(p.protocol)}
                    </MenuItem>
                  ))}
                </TextField>
                <TextField
                  label="Source"
                  value={sourceFilter}
                  onChange={(e) => {
                    setSourceFilter(e.target.value);
                    scheduleDebounce(setAppliedSource, e.target.value);
                  }}
                  size="small"
                  sx={{ minWidth: 140 }}
                  placeholder="e.g. 1.2.3.4:443"
                />
                <TextField
                  label="Destination"
                  value={destinationFilter}
                  onChange={(e) => {
                    setDestinationFilter(e.target.value);
                    scheduleDebounce(setAppliedDestination, e.target.value);
                  }}
                  size="small"
                  sx={{ minWidth: 140 }}
                  placeholder="e.g. 1.2.3.4:443"
                />
              </Box>

              {/* Flow table */}
              {flowRowCount === 0 && !isFlowLoading ? (
                <EmptyState
                  primaryText="No flow reports found"
                  secondaryText="No flows match the current filters, or flow accounting has not recorded any data yet."
                  button={false}
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
                    disableColumnSorting
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
