import React, { useEffect, useMemo } from "react";
import {
  Box,
  Typography,
  CircularProgress,
  Card,
  CardHeader,
  CardContent,
  CardActionArea,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Paper,
  Tooltip,
} from "@mui/material";
import Grid from "@mui/material/Grid";
import { useTheme } from "@mui/material/styles";
import EastIcon from "@mui/icons-material/East";
import WestIcon from "@mui/icons-material/West";
import { PieChart } from "@mui/x-charts/PieChart";
import { Link, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { getStatus, type APIStatus } from "@/queries/status";
import ClusterSummaryCard from "@/components/ClusterSummaryCard";
import { getMetrics } from "@/queries/metrics";
import {
  listSubscribers,
  type ListSubscribersResponse,
} from "@/queries/subscribers";
import { listRadios, type ListRadiosResponse } from "@/queries/radios";
import {
  listRadioEvents,
  type ListRadioEventsResponse,
} from "@/queries/radio_events";
import {
  getFlowReportStats,
  type FlowReportStatsResponse,
} from "@/queries/flow_reports";
import { getUsage, type UsageResult } from "@/queries/usage";
import {
  formatDateTime,
  formatBytesAutoUnit,
  formatProtocol,
  PIE_COLORS,
} from "@/utils/formatters";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const nf = new Intl.NumberFormat();
const formatNumber = (n: number | null | undefined) =>
  n == null ? "N/A" : nf.format(n);

const formatMemory = (value: number | null | undefined): string => {
  if (value == null || !Number.isFinite(value)) return "N/A";

  const base = 1024;
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];

  let i = 0;
  let n = Math.abs(value);
  while (n >= base && i < units.length - 1) {
    n /= base;
    i++;
  }

  const numFmt = new Intl.NumberFormat("en-US", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  });

  const sign = value < 0 ? "-" : "";
  return `${sign}${numFmt.format(n)} ${units[i]}`;
};

// ──────────────────────────────────────────────────────
// Metrics parsing
// ──────────────────────────────────────────────────────

type ParsedMetrics = {
  pduSessions: number | null;
  heapMemoryBytes: number | null;
  totalMemoryBytes: number | null;
  databaseSizeBytes: number | null;
  routines: number | null;
  allocatedIPs: number | null;
  totalIPs: number | null;
  processStart: number | null;
};

const parseMetrics = (raw: string): ParsedMetrics => {
  const map = new Map<string, number>();
  for (const line of raw.split("\n")) {
    if (line.startsWith("#") || line === "") continue;
    const spaceIdx = line.lastIndexOf(" ");
    if (spaceIdx === -1) continue;
    const key = line.slice(0, spaceIdx + 1);
    const n = Number(line.slice(spaceIdx + 1));
    if (Number.isFinite(n)) map.set(key, n);
  }

  const g = (k: string) => map.get(k) ?? null;

  return {
    pduSessions: g("app_pdu_sessions_total "),
    heapMemoryBytes: g("go_memstats_heap_inuse_bytes "),
    totalMemoryBytes: g("process_resident_memory_bytes "),
    databaseSizeBytes: g("app_database_storage_bytes "),
    routines: g("go_goroutines "),
    allocatedIPs:
      g("app_ip_addresses_allocated_total ") != null
        ? Math.round(g("app_ip_addresses_allocated_total ")!)
        : null,
    totalIPs:
      g("app_ip_addresses_total ") != null
        ? Math.round(g("app_ip_addresses_total ")!)
        : null,
    processStart: g("process_start_time_seconds "),
  };
};

// ──────────────────────────────────────────────────────
// KpiCard
// ──────────────────────────────────────────────────────

type KpiCardProps = {
  title: React.ReactNode;
  value?: React.ReactNode;
  loading?: boolean;
  error?: boolean;
  onClick?: () => void;
  children?: React.ReactNode;
  minHeight?: number;
};

function KpiCard({
  title,
  value,
  loading,
  error,
  onClick,
  children,
  minHeight = 200,
}: KpiCardProps) {
  const body =
    children ??
    (loading ? (
      <Skeleton width={120} height={40} />
    ) : error ? (
      <Typography variant="body2" color="error">
        Failed to load data.
      </Typography>
    ) : (
      <Typography variant="h4">{value}</Typography>
    ));

  const CardInner = (
    <Card
      sx={{
        height: "100%",
        display: "flex",
        flexDirection: "column",
        borderRadius: 3,
        boxShadow: 2,
      }}
    >
      <CardHeader
        title={title}
        sx={{
          backgroundColor: "backgroundSubtle",
          borderTopLeftRadius: 8,
          borderTopRightRadius: 8,
          "& .MuiCardHeader-title": {
            fontWeight: 600,
            fontSize: "1rem",
          },
        }}
      />
      <CardContent
        sx={{
          flexGrow: 1,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          minHeight: minHeight,
        }}
      >
        {body}
      </CardContent>
    </Card>
  );

  if (onClick) {
    return (
      <CardActionArea
        onClick={onClick}
        sx={{ height: "100%", borderRadius: 3 }}
      >
        {CardInner}
      </CardActionArea>
    );
  }

  return CardInner;
}

// ──────────────────────────────────────────────────────
// Date helpers (for usage query — last 7 days)
// ──────────────────────────────────────────────────────

const getDefaultDateRange = () => {
  const today = new Date();
  const sevenDaysAgo = new Date();
  sevenDaysAgo.setDate(today.getDate() - 6);
  const format = (d: Date) => d.toISOString().slice(0, 10);
  return { startDate: format(sevenDaysAgo), endDate: format(today) };
};

// ──────────────────────────────────────────────────────
// Dashboard
// ──────────────────────────────────────────────────────

const Dashboard = () => {
  const navigate = useNavigate();
  const theme = useTheme();
  const { accessToken, authReady } = useAuth();
  const { startDate, endDate } = getDefaultDateRange();

  // ── Queries ─────────────────────────────────────────

  const statusQuery = useQuery<APIStatus>({
    queryKey: ["dashboardStatus"],
    queryFn: () => getStatus(),
    enabled: authReady,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const subscribersQuery = useQuery<ListSubscribersResponse>({
    queryKey: ["dashboardSubscribers"],
    queryFn: () => listSubscribers(accessToken!, 1, 1),
    enabled: authReady && !!accessToken,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const radiosQuery = useQuery<ListRadiosResponse>({
    queryKey: ["dashboardRadios"],
    queryFn: () => listRadios(accessToken!, 1, 1),
    enabled: authReady && !!accessToken,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const metricsQuery = useQuery<ParsedMetrics>({
    queryKey: ["dashboardMetrics"],
    queryFn: async () => parseMetrics(await getMetrics()),
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const radioEventsQuery = useQuery<ListRadioEventsResponse>({
    queryKey: ["dashboardRadioEvents"],
    queryFn: () => listRadioEvents(accessToken!, 1, 10),
    enabled: authReady && !!accessToken,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const flowStatsQuery = useQuery<FlowReportStatsResponse>({
    queryKey: ["dashboardFlowStats", startDate, endDate],
    queryFn: () =>
      // Do not force 'allow' so the dashboard shows both allowed and dropped
      // flows by default. The API returns both when action is omitted.
      getFlowReportStats(accessToken!, {
        start: startDate,
        end: endDate,
      }),
    enabled: authReady && !!accessToken,
    refetchInterval: 10000,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const usageQuery = useQuery<UsageResult>({
    queryKey: ["dashboardUsage", startDate, endDate],
    queryFn: () => getUsage(accessToken!, startDate, endDate, "", "subscriber"),
    enabled: authReady && !!accessToken,
    refetchInterval: 10000,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  // ── Derived values ──────────────────────────────────

  const version = statusQuery.data?.version ?? null;
  const subscriberCount = subscribersQuery.data?.total_count ?? null;
  const radioCount = radiosQuery.data?.total_count ?? null;
  const m = metricsQuery.data;
  const networkLogs = radioEventsQuery.data?.items ?? [];
  const metricsLoading = metricsQuery.isLoading;
  const radiosLoading = radiosQuery.isLoading;
  const subscribersLoading = subscribersQuery.isLoading;
  const eventsLoading = radioEventsQuery.isLoading;
  const statusLoading = statusQuery.isLoading;

  const { showSnackbar } = useSnackbar();

  useEffect(() => {
    if (statusQuery.error || subscribersQuery.error || metricsQuery.error) {
      showSnackbar("Failed to fetch dashboard data.", "error");
    }
  }, [
    statusQuery.error,
    subscribersQuery.error,
    metricsQuery.error,
    showSnackbar,
  ]);

  useEffect(() => {
    if (radioEventsQuery.error) {
      showSnackbar("Failed to fetch radio events.", "error");
    }
  }, [radioEventsQuery.error, showSnackbar]);

  const activeSessions = m?.pduSessions ?? null;
  const heapMemory = m?.heapMemoryBytes ?? null;
  const totalMemory = m?.totalMemoryBytes ?? null;
  const databaseSize = m?.databaseSizeBytes ?? null;
  const routines = m?.routines ?? null;
  const allocatedIPs = m?.allocatedIPs ?? null;
  const totalIPs = m?.totalIPs ?? null;
  const upSince = m?.processStart ? new Date(m.processStart * 1000) : null;

  const ipChart = useMemo(() => {
    const alloc = allocatedIPs ?? 0;
    const total = totalIPs ?? 0;
    const available = Math.max(total - alloc, 0);
    return { alloc, available, total };
  }, [allocatedIPs, totalIPs]);

  // ── Protocol donut data ─────────────────────────────

  const protocolPieData = useMemo(() => {
    if (!flowStatsQuery.data?.protocols?.length) return [];
    return flowStatsQuery.data.protocols.map((p, i) => ({
      id: p.protocol,
      value: p.count,
      label: formatProtocol(p.protocol),
      color: PIE_COLORS[i % PIE_COLORS.length],
    }));
  }, [flowStatsQuery.data]);

  // ── Top 10 data users ───────────────────────────────

  type TopUser = {
    id: string;
    subscriber: string;
    total_bytes: number;
    uplink_bytes: number;
    downlink_bytes: number;
  };

  const topUsers: TopUser[] = useMemo(() => {
    if (!usageQuery.data) return [];
    const items: TopUser[] = [];
    for (const entry of usageQuery.data) {
      const subscriber = Object.keys(entry)[0];
      const usage = entry[subscriber];
      if (!subscriber || !usage) continue;
      items.push({
        id: subscriber,
        subscriber,
        total_bytes: usage.total_bytes,
        uplink_bytes: usage.uplink_bytes,
        downlink_bytes: usage.downlink_bytes,
      });
    }
    items.sort((a, b) => b.total_bytes - a.total_bytes);
    return items.slice(0, 10);
  }, [usageQuery.data]);

  // ── Render ──────────────────────────────────────────

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <Box
        sx={{
          mb: 3,
          display: "flex",
          flexWrap: "wrap",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 2,
        }}
      >
        <Typography variant="h4" component="h1">
          Ella Core{" "}
          {statusLoading ? (
            <CircularProgress size={22} sx={{ ml: 1 }} />
          ) : (
            (version ?? "—")
          )}
        </Typography>
      </Box>

      {statusQuery.data?.cluster?.enabled && (
        <Box sx={{ mb: 4 }}>
          <ClusterSummaryCard />
        </Box>
      )}

      {/* ─── 1. Network Status ──────────────────────── */}
      <Typography variant="h5" component="h2" sx={{ mb: 2 }}>
        Network Status
      </Typography>

      <Grid
        container
        spacing={4}
        sx={{ alignItems: "stretch", justifyContent: "flex-start" }}
      >
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Tooltip
            title="Number of radio base stations (gNBs) connected to this core"
            arrow
          >
            <Box>
              <KpiCard
                title="Radios"
                loading={radiosLoading}
                error={!!radiosQuery.error}
                value={formatNumber(radioCount)}
                onClick={() => navigate("/radios")}
              />
            </Box>
          </Tooltip>
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Tooltip
            title="Total number of subscribers provisioned in the network"
            arrow
          >
            <Box>
              <KpiCard
                title="Subscribers"
                loading={subscribersLoading}
                error={!!subscribersQuery.error}
                value={formatNumber(subscriberCount)}
                onClick={() => navigate("/subscribers")}
              />
            </Box>
          </Tooltip>
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Tooltip
            title="Number of active PDU sessions (devices currently connected with data sessions)"
            arrow
          >
            <Box>
              <KpiCard
                title="Active Sessions"
                loading={metricsLoading}
                error={!!metricsQuery.error}
                value={formatNumber(activeSessions)}
              />
            </Box>
          </Tooltip>
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Tooltip title="Time when the core process was started" arrow>
            <Box>
              <KpiCard
                title="Up Since"
                loading={metricsLoading}
                error={!!metricsQuery.error}
                value={upSince ? formatDateTime(upSince.toISOString()) : "N/A"}
              />
            </Box>
          </Tooltip>
        </Grid>
      </Grid>

      {/* ─── 2. Control Plane ───────────────────────── */}
      <Typography variant="h5" component="h2" sx={{ mt: 4, mb: 2 }}>
        Control Plane
      </Typography>

      <Grid
        container
        spacing={4}
        sx={{ alignItems: "stretch", justifyContent: "flex-start" }}
      >
        <Grid size={{ xs: 12, sm: 12, md: 4 }}>
          <KpiCard
            title="IP Allocation"
            loading={metricsLoading}
            error={!!metricsQuery.error}
            minHeight={240}
          >
            {metricsLoading ? (
              <Skeleton variant="rounded" width="100%" height={200} />
            ) : (
              <Box sx={{ width: "100%", height: 220 }}>
                <PieChart
                  series={[
                    {
                      data: [
                        { id: 0, value: ipChart.alloc, label: "Allocated" },
                        { id: 1, value: ipChart.available, label: "Available" },
                      ],
                      innerRadius: 40,
                      outerRadius: 100,
                      paddingAngle: 2,
                      cornerRadius: 5,
                    },
                  ]}
                  height={220}
                  width={undefined}
                />
                <Typography
                  variant="body2"
                  color="textSecondary"
                  sx={{ mt: 1 }}
                >
                  {ipChart.total > 0
                    ? `${Math.round((ipChart.alloc / ipChart.total) * 100)}% used`
                    : "N/A"}
                </Typography>
              </Box>
            )}
          </KpiCard>
        </Grid>
        <Grid size={{ xs: 12, sm: 12, md: 8 }}>
          <KpiCard
            title={
              <Box
                component={Link}
                to="/radios/events"
                sx={{
                  textDecoration: "none",
                  color: "inherit",
                  "&:hover": { textDecoration: "underline" },
                  cursor: "pointer",
                }}
              >
                Recent Network Events
              </Box>
            }
            loading={eventsLoading}
            minHeight={240}
          >
            {radioEventsQuery.error ? (
              <Typography color="error" sx={{ p: 2 }}>
                Failed to fetch radio events.
              </Typography>
            ) : (
              <TableContainer
                component={Paper}
                elevation={0}
                sx={{
                  width: "100%",
                  maxHeight: 220,
                  overflowY: "auto",
                }}
              >
                <Table
                  size="small"
                  stickyHeader
                  aria-label="recent-network-events"
                >
                  <TableHead>
                    <TableRow>
                      <TableCell
                        sx={{
                          fontWeight: 600,
                          width: 150,
                          whiteSpace: "nowrap",
                        }}
                      >
                        Timestamp
                      </TableCell>

                      <TableCell sx={{ fontWeight: 600, whiteSpace: "nowrap" }}>
                        Radio
                      </TableCell>

                      <TableCell sx={{ fontWeight: 600, minWidth: 220 }}>
                        Message Type
                      </TableCell>

                      <TableCell
                        sx={{
                          fontWeight: 600,
                          whiteSpace: "nowrap",
                          width: 90,
                          textAlign: "center",
                        }}
                      >
                        Direction
                      </TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {(networkLogs ?? []).slice(0, 10).map((row) => {
                      const dirIcon =
                        row.direction === "outbound" ? (
                          <EastIcon
                            fontSize="small"
                            sx={{ color: theme.palette.info.main }}
                          />
                        ) : row.direction === "inbound" ? (
                          <WestIcon
                            fontSize="small"
                            sx={{ color: theme.palette.success.main }}
                          />
                        ) : null;
                      const dirTitle =
                        row.direction === "inbound"
                          ? "Receive (inbound)"
                          : row.direction === "outbound"
                            ? "Send (outbound)"
                            : "";
                      return (
                        <TableRow key={row.id} hover>
                          <TableCell sx={{ whiteSpace: "nowrap" }}>
                            {formatDateTime(row.timestamp, { seconds: true })}
                          </TableCell>

                          <TableCell sx={{ whiteSpace: "nowrap" }}>
                            {row.radio ? (
                              <Link
                                to={`/radios/${encodeURIComponent(row.radio)}`}
                                style={{ textDecoration: "none" }}
                              >
                                <Typography
                                  variant="body2"
                                  sx={{
                                    color: theme.palette.link,
                                    textDecoration: "underline",
                                    "&:hover": { textDecoration: "underline" },
                                  }}
                                >
                                  {row.radio}
                                </Typography>
                              </Link>
                            ) : (
                              <Typography variant="body2" color="textSecondary">
                                —
                              </Typography>
                            )}
                          </TableCell>

                          <TableCell
                            sx={{
                              overflow: "hidden",
                              textOverflow: "ellipsis",
                            }}
                            title={row.message_type}
                          >
                            {row.message_type}
                          </TableCell>

                          <TableCell sx={{ textAlign: "center" }}>
                            {dirIcon && (
                              <Tooltip title={dirTitle}>{dirIcon}</Tooltip>
                            )}
                          </TableCell>
                        </TableRow>
                      );
                    })}
                    {(!networkLogs || networkLogs.length === 0) && (
                      <TableRow>
                        <TableCell colSpan={4}>
                          <Typography variant="body2" color="textSecondary">
                            No network events.
                          </Typography>
                        </TableCell>
                      </TableRow>
                    )}
                  </TableBody>
                </Table>
              </TableContainer>
            )}
          </KpiCard>
        </Grid>
      </Grid>

      {/* ─── 3. User Plane ──────────────────────────── */}
      <Typography variant="h5" component="h2" sx={{ mt: 4, mb: 2 }}>
        User Plane
      </Typography>

      <Grid
        container
        spacing={4}
        sx={{ alignItems: "stretch", justifyContent: "flex-start" }}
      >
        <Grid size={{ xs: 12, sm: 12, md: 4 }}>
          <KpiCard
            title={
              <Box
                component={Link}
                to="/traffic/flows"
                sx={{
                  textDecoration: "none",
                  color: "inherit",
                  "&:hover": { textDecoration: "underline" },
                  cursor: "pointer",
                }}
              >
                Flow Protocols (Last 7 days)
              </Box>
            }
            loading={flowStatsQuery.isLoading}
            minHeight={240}
          >
            {flowStatsQuery.isLoading ? (
              <Skeleton variant="rounded" width="100%" height={200} />
            ) : protocolPieData.length === 0 ? (
              <Typography variant="body2" color="textSecondary">
                No flow data available.
              </Typography>
            ) : (
              <Box sx={{ width: "100%", height: 220 }}>
                <PieChart
                  series={[
                    {
                      data: protocolPieData,
                      innerRadius: 40,
                      outerRadius: 100,
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
                  height={220}
                  width={undefined}
                />
              </Box>
            )}
          </KpiCard>
        </Grid>
        <Grid size={{ xs: 12, sm: 12, md: 8 }}>
          <KpiCard
            title={
              <Box
                component={Link}
                to="/traffic/usage"
                sx={{
                  textDecoration: "none",
                  color: "inherit",
                  "&:hover": { textDecoration: "underline" },
                  cursor: "pointer",
                }}
              >
                Top 10 Data Users (Last 7 days)
              </Box>
            }
            loading={usageQuery.isLoading}
            minHeight={240}
          >
            {usageQuery.isLoading ? (
              <Skeleton variant="rounded" width="100%" height={200} />
            ) : topUsers.length === 0 ? (
              <Typography variant="body2" color="textSecondary">
                No usage data available.
              </Typography>
            ) : (
              <TableContainer
                component={Paper}
                elevation={0}
                sx={{
                  width: "100%",
                  maxHeight: 220,
                  overflowY: "auto",
                }}
              >
                <Table size="small" stickyHeader aria-label="top-data-users">
                  <TableHead>
                    <TableRow>
                      <TableCell sx={{ fontWeight: 600, minWidth: 160 }}>
                        Subscriber
                      </TableCell>
                      <TableCell sx={{ fontWeight: 600 }} align="right">
                        Downlink
                      </TableCell>
                      <TableCell sx={{ fontWeight: 600 }} align="right">
                        Uplink
                      </TableCell>
                      <TableCell sx={{ fontWeight: 600 }} align="right">
                        Total
                      </TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {topUsers.map((row) => (
                      <TableRow key={row.id} hover>
                        <TableCell>
                          <Link
                            to={`/subscribers/${encodeURIComponent(row.subscriber)}`}
                            style={{ textDecoration: "none" }}
                          >
                            <Typography
                              variant="body2"
                              sx={{
                                fontFamily: "monospace",
                                color: theme.palette.link,
                                textDecoration: "underline",
                                "&:hover": { textDecoration: "underline" },
                              }}
                            >
                              {row.subscriber}
                            </Typography>
                          </Link>
                        </TableCell>
                        <TableCell align="right">
                          {formatBytesAutoUnit(row.downlink_bytes)}
                        </TableCell>
                        <TableCell align="right">
                          {formatBytesAutoUnit(row.uplink_bytes)}
                        </TableCell>
                        <TableCell align="right">
                          {formatBytesAutoUnit(row.total_bytes)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            )}
          </KpiCard>
        </Grid>
      </Grid>

      {/* ─── 4. System ──────────────────────────────── */}
      <Typography variant="h5" component="h2" sx={{ mt: 4, mb: 2 }}>
        System
      </Typography>

      <Grid
        container
        spacing={4}
        sx={{ alignItems: "stretch", justifyContent: "flex-start" }}
      >
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Tooltip
            title="Memory allocated on the heap for the application"
            arrow
          >
            <Box>
              <KpiCard
                title="Heap Memory"
                loading={metricsLoading}
                error={!!metricsQuery.error}
                value={formatMemory(heapMemory)}
              />
            </Box>
          </Tooltip>
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Tooltip title="Total physical RAM used by the core process" arrow>
            <Box>
              <KpiCard
                title="Total Memory"
                loading={metricsLoading}
                error={!!metricsQuery.error}
                value={formatMemory(totalMemory)}
              />
            </Box>
          </Tooltip>
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Tooltip title="Size of the database file on disk" arrow>
            <Box>
              <KpiCard
                title="Database Size"
                loading={metricsLoading}
                error={!!metricsQuery.error}
                value={formatMemory(databaseSize)}
              />
            </Box>
          </Tooltip>
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Tooltip
            title="Number of concurrent tasks currently running in the process"
            arrow
          >
            <Box>
              <KpiCard
                title="Routines"
                loading={metricsLoading}
                error={!!metricsQuery.error}
                value={routines != null ? `${routines}` : "N/A"}
              />
            </Box>
          </Tooltip>
        </Grid>
      </Grid>
    </Box>
  );
};

export default Dashboard;
