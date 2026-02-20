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

const MAX_WIDTH = 1200;

const nf = new Intl.NumberFormat();
const formatNumber = (n: number | null | undefined) =>
  n == null ? "N/A" : nf.format(n);

const formatBytes = (value: number | null | undefined): string => {
  if (value == null || !Number.isFinite(value)) return "N/A";

  const base = 1000;
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];

  let i = 0;
  let n = Math.abs(value);
  while (n >= base && i < units.length - 1) {
    n /= base;
    i++;
  }

  const nf = new Intl.NumberFormat("en-US", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  });

  const sign = value < 0 ? "-" : "";
  return `${sign}${nf.format(n)} ${units[i]}`;
};

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

  const nf = new Intl.NumberFormat("en-US", {
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
  });

  const sign = value < 0 ? "-" : "";
  return `${sign}${nf.format(n)} ${units[i]}`;
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

type ParsedMetrics = {
  pduSessions: number | null;
  heapMemoryBytes: number | null;
  totalMemoryBytes: number | null;
  databaseSizeBytes: number | null;
  routines: number | null;
  allocatedIPs: number | null;
  totalIPs: number | null;
  uplinkBytes: number | null;
  downlinkBytes: number | null;
  n3Drops: number | null;
  n6Drops: number | null;
  n3Pass: number | null;
  n6Pass: number | null;
  n3Tx: number | null;
  n6Tx: number | null;
  n3Redirect: number | null;
  n6Redirect: number | null;
  n3Aborted: number | null;
  n6Aborted: number | null;
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
    uplinkBytes: g("app_uplink_bytes "),
    downlinkBytes: g("app_downlink_bytes "),
    n3Drops: g('app_xdp_action_total{action="XDP_DROP",interface="n3"} '),
    n6Drops: g('app_xdp_action_total{action="XDP_DROP",interface="n6"} '),
    n3Pass: g('app_xdp_action_total{action="XDP_PASS",interface="n3"} '),
    n6Pass: g('app_xdp_action_total{action="XDP_PASS",interface="n6"} '),
    n3Tx: g('app_xdp_action_total{action="XDP_TX",interface="n3"} '),
    n6Tx: g('app_xdp_action_total{action="XDP_TX",interface="n6"} '),
    n3Redirect: g(
      'app_xdp_action_total{action="XDP_REDIRECT",interface="n3"} ',
    ),
    n6Redirect: g(
      'app_xdp_action_total{action="XDP_REDIRECT",interface="n6"} ',
    ),
    n3Aborted: g('app_xdp_action_total{action="XDP_ABORTED",interface="n3"} '),
    n6Aborted: g('app_xdp_action_total{action="XDP_ABORTED",interface="n6"} '),
    processStart: g("process_start_time_seconds "),
  };
};

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

const Dashboard = () => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

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

  const version = statusQuery.data?.version ?? null;
  const subscriberCount = subscribersQuery.data?.total_count ?? null;
  const radioCount = radiosQuery.data?.total_count ?? null;
  const m = metricsQuery.data;
  const networkLogs = radioEventsQuery.data?.items ?? [];
  const logsError = radioEventsQuery.error
    ? "Failed to fetch radio events."
    : null;

  const loading = metricsQuery.isLoading;
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
  const uplinkBytes = m?.uplinkBytes ?? null;
  const downlinkBytes = m?.downlinkBytes ?? null;
  const n3Drops = m?.n3Drops ?? null;
  const n6Drops = m?.n6Drops ?? null;
  const n3Pass = m?.n3Pass ?? null;
  const n6Pass = m?.n6Pass ?? null;
  const n3Tx = m?.n3Tx ?? null;
  const n6Tx = m?.n6Tx ?? null;
  const n3Redirect = m?.n3Redirect ?? null;
  const n6Redirect = m?.n6Redirect ?? null;
  const n3Aborted = m?.n3Aborted ?? null;
  const n6Aborted = m?.n6Aborted ?? null;
  const upSince = m?.processStart ? new Date(m.processStart * 1000) : null;

  const ipChart = useMemo(() => {
    const alloc = allocatedIPs ?? 0;
    const total = totalIPs ?? 0;
    const available = Math.max(total - alloc, 0);
    return { alloc, available, total };
  }, [allocatedIPs, totalIPs]);

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
          {loading ? (
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

      <Typography variant="h5" component="h2" sx={{ mb: 2 }}>
        Network
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
                loading={loading}
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
                loading={loading}
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
                loading={loading}
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
                loading={loading}
                value={upSince ? formatTimestamp(upSince.toISOString()) : "N/A"}
              />
            </Box>
          </Tooltip>
        </Grid>
        <Grid size={{ xs: 12, sm: 12, md: 4 }}>
          <KpiCard title="IP Allocation" loading={loading} minHeight={240}>
            {loading ? (
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
            loading={loading}
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
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Tooltip
            title="Total uplink traffic from devices (N3 → N6) since core started"
            arrow
          >
            <Box>
              <KpiCard
                title="Uplink Traffic"
                loading={loading}
                value={formatBytes(uplinkBytes)}
              />
            </Box>
          </Tooltip>
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Tooltip
            title="Total downlink traffic to devices (N6 → N3) since core started"
            arrow
          >
            <Box>
              <KpiCard
                title="Downlink Traffic"
                loading={loading}
                value={formatBytes(downlinkBytes)}
              />
            </Box>
          </Tooltip>
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Tooltip
            title="Packets dropped by the eBPF program on the N3 interface"
            arrow
          >
            <Box>
              <KpiCard title="Uplink Drops" loading={loading}>
                {loading ? (
                  <Skeleton width={120} height={40} />
                ) : (
                  (() => {
                    const totalN3 =
                      (n3Drops ?? 0) +
                      (n3Pass ?? 0) +
                      (n3Tx ?? 0) +
                      (n3Redirect ?? 0) +
                      (n3Aborted ?? 0);
                    return totalN3 > 0 && n3Drops != null ? (
                      <Box sx={{ textAlign: "center" }}>
                        <Typography variant="h4">
                          {((n3Drops / totalN3) * 100).toFixed(3)}%
                        </Typography>
                        <Typography
                          variant="body2"
                          color="text.secondary"
                          sx={{ mt: 0.5 }}
                        >
                          {formatNumber(n3Drops)} packets
                        </Typography>
                      </Box>
                    ) : (
                      <Typography variant="h4">N/A</Typography>
                    );
                  })()
                )}
              </KpiCard>
            </Box>
          </Tooltip>
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <Tooltip
            title="Packets dropped by the eBPF program on the N6 interface (data network)"
            arrow
          >
            <Box>
              <KpiCard title="Downlink Drops" loading={loading}>
                {loading ? (
                  <Skeleton width={120} height={40} />
                ) : (
                  (() => {
                    const totalN6 =
                      (n6Drops ?? 0) +
                      (n6Pass ?? 0) +
                      (n6Tx ?? 0) +
                      (n6Redirect ?? 0) +
                      (n6Aborted ?? 0);
                    return totalN6 > 0 && n6Drops != null ? (
                      <Box sx={{ textAlign: "center" }}>
                        <Typography variant="h4">
                          {((n6Drops / totalN6) * 100).toFixed(3)}%
                        </Typography>
                        <Typography
                          variant="body2"
                          color="text.secondary"
                          sx={{ mt: 0.5 }}
                        >
                          {formatNumber(n6Drops)} packets
                        </Typography>
                      </Box>
                    ) : (
                      <Typography variant="h4">N/A</Typography>
                    );
                  })()
                )}
              </KpiCard>
            </Box>
          </Tooltip>
        </Grid>
      </Grid>

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
                loading={loading}
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
                loading={loading}
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
                loading={loading}
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
                loading={loading}
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
