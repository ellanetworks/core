import React, { useMemo } from "react";
import {
  Box,
  Typography,
  CircularProgress,
  Alert,
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
import { PieChart } from "@mui/x-charts/PieChart";
import { Link, useNavigate } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useAuth } from "@/contexts/AuthContext";
import { getStatus, type APIStatus } from "@/queries/status";
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
  listFlowReports,
  type FlowReport,
  type ListFlowReportsResponse,
} from "@/queries/flow_reports";
import { getUsage, type UsageResult } from "@/queries/usage";

const MAX_WIDTH = 1400;

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

const formatBytesAutoUnit = (bytes: number): string => {
  if (!Number.isFinite(bytes)) return "";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let n = Math.abs(bytes);
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  const decimals = n >= 100 ? 0 : n >= 10 ? 1 : 2;
  return `${n.toFixed(decimals)} ${units[i]}`;
};

const formatTimestamp = (s: string) => {
  if (!s) return "";
  const d = new Date(s);
  if (isNaN(d.getTime())) {
    return s.replace(/\s*[+-]\d{4}$/, "");
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
// Protocol helpers
// ──────────────────────────────────────────────────────

const PROTOCOL_NAMES: Record<number, string> = {
  1: "ICMP", 6: "TCP", 17: "UDP", 41: "IPv6", 47: "GRE",
  50: "ESP", 51: "AH", 58: "IPv6-ICMP", 89: "OSPFIGP",
  132: "SCTP",
};

const formatProtocol = (value: number): string =>
  PROTOCOL_NAMES[value] ?? String(value);

// ──────────────────────────────────────────────────────
// Pie chart color palette
// ──────────────────────────────────────────────────────

const PIE_COLORS = [
  "#2196F3", "#4CAF50", "#FF9800", "#E91E63", "#9C27B0",
  "#00BCD4", "#FF5722", "#795548", "#607D8B", "#8BC34A",
  "#3F51B5", "#CDDC39",
];

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
  onClick?: () => void;
  children?: React.ReactNode;
  minHeight?: number;
};

function KpiCard({
  title,
  value,
  loading,
  onClick,
  children,
  minHeight = 200,
}: KpiCardProps) {
  const body =
    children ??
    (loading ? (
      <Skeleton width={120} height={40} />
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
          backgroundColor: "#F5F5F5",
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

  const flowQuery = useQuery<ListFlowReportsResponse>({
    queryKey: ["dashboardFlows"],
    queryFn: () =>
      listFlowReports(accessToken!, 1, 100, {
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
    queryFn: () =>
      getUsage(accessToken!, startDate, endDate, "", "subscriber"),
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
  const logsError = radioEventsQuery.error
    ? "Failed to fetch radio events."
    : null;

  const metricsLoading = metricsQuery.isLoading;
  const radiosLoading = radiosQuery.isLoading;
  const subscribersLoading = subscribersQuery.isLoading;
  const eventsLoading = radioEventsQuery.isLoading;
  const statusLoading = statusQuery.isLoading;
  const error =
    statusQuery.error || subscribersQuery.error || metricsQuery.error
      ? "Failed to fetch dashboard data."
      : null;

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

  const flowRows: FlowReport[] = flowQuery.data?.items ?? [];

  const protocolPieData = useMemo(() => {
    if (!flowRows.length) return [];
    const counts = new Map<number, number>();
    for (const row of flowRows) {
      counts.set(row.protocol, (counts.get(row.protocol) ?? 0) + 1);
    }
    const sorted = [...counts.entries()].sort((a, b) => b[1] - a[1]);
    return sorted.map(([proto, count], i) => ({
      id: proto,
      value: count,
      label: formatProtocol(proto),
      color: PIE_COLORS[i % PIE_COLORS.length],
    }));
  }, [flowRows]);

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
    <Box sx={{ px: { xs: 2, sm: 4 }, py: 3, maxWidth: MAX_WIDTH, mx: "auto" }}>
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

      {error && (
        <Alert severity="error" sx={{ mb: 3 }}>
          {error}
        </Alert>
      )}

      {/* ─── 1. Network Status ──────────────────────── */}
      <Typography variant="h5" component="h2" sx={{ mb: 2 }}>
        Network Status
      </Typography>

      <Grid
        container
        spacing={4}
        alignItems="stretch"
        justifyContent="flex-start"
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
                value={upSince ? formatTimestamp(upSince.toISOString()) : "N/A"}
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
        alignItems="stretch"
        justifyContent="flex-start"
      >
        <Grid size={{ xs: 12, sm: 12, md: 4 }}>
          <KpiCard title="IP Allocation" loading={metricsLoading} minHeight={240}>
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
                      paddingAngle: 0,
                      cornerRadius: 0,
                    },
                  ]}
                  height={220}
                  width={undefined}
                />
                <Typography
                  variant="body2"
                  color="text.secondary"
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
                to="/radios?tab=events"
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
            {logsError ? (
              <Alert severity="error" sx={{ width: "100%" }}>
                {logsError}
              </Alert>
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
                        Protocol
                      </TableCell>

                      <TableCell sx={{ fontWeight: 600, minWidth: 220 }}>
                        Message Type
                      </TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {(networkLogs ?? []).slice(0, 10).map((row) => (
                      <TableRow key={row.id} hover>
                        <TableCell sx={{ whiteSpace: "nowrap" }}>
                          {formatTimestamp(row.timestamp)}
                        </TableCell>

                        <TableCell sx={{ whiteSpace: "nowrap" }}>
                          {row.protocol}
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
                      </TableRow>
                    ))}
                    {(!networkLogs || networkLogs.length === 0) && (
                      <TableRow>
                        <TableCell colSpan={3}>
                          <Typography variant="body2" color="text.secondary">
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
        alignItems="stretch"
        justifyContent="flex-start"
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
                Flow Protocols
              </Box>
            }
            loading={flowQuery.isLoading}
            minHeight={240}
          >
            {flowQuery.isLoading ? (
              <Skeleton variant="rounded" width="100%" height={200} />
            ) : protocolPieData.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
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
                      paddingAngle: 0,
                      cornerRadius: 0,
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
                Top 10 Data Users
              </Box>
            }
            loading={usageQuery.isLoading}
            minHeight={240}
          >
            {usageQuery.isLoading ? (
              <Skeleton variant="rounded" width="100%" height={200} />
            ) : topUsers.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
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
                <Table
                  size="small"
                  stickyHeader
                  aria-label="top-data-users"
                >
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
                        <TableCell>{row.subscriber}</TableCell>
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
        alignItems="stretch"
        justifyContent="flex-start"
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
