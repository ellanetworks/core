"use client";

import React, { useEffect, useMemo, useState } from "react";
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
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";
import { getStatus } from "@/queries/status";
import { getMetrics } from "@/queries/metrics";
import {
  listSubscribers,
  type ListSubscribersResponse,
} from "@/queries/subscribers";
import { listRadios } from "@/queries/radios";
import {
  listRadioEvents,
  type APIRadioEvent,
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
  const router = useRouter();
  const { accessToken, authReady } = useAuth();

  const [version, setVersion] = useState<string | null>(null);
  const [subscriberCount, setSubscriberCount] = useState<number | null>(null);
  const [radioCount, setRadioCount] = useState<number | null>(null);

  const [activeSessions, setActiveSessions] = useState<number | null>(null);
  const [heapMemory, setHeapMemory] = useState<number | null>(null);
  const [totalMemory, setTotalMemory] = useState<number | null>(null);
  const [databaseSize, setDatabaseSize] = useState<number | null>(null);
  const [routines, setRoutines] = useState<number | null>(null);
  const [allocatedIPs, setAllocatedIPs] = useState<number | null>(null);
  const [totalIPs, setTotalIPs] = useState<number | null>(null);

  const [uplinkBytes, setUplinkBytes] = useState<number | null>(null);
  const [downlinkBytes, setDownlinkBytes] = useState<number | null>(null);
  const [n3Drops, setN3Drops] = useState<number | null>(null);
  const [n6Drops, setN6Drops] = useState<number | null>(null);
  const [n3Pass, setN3Pass] = useState<number | null>(null);
  const [n6Pass, setN6Pass] = useState<number | null>(null);
  const [n3Tx, setN3Tx] = useState<number | null>(null);
  const [n6Tx, setN6Tx] = useState<number | null>(null);
  const [n3Redirect, setN3Redirect] = useState<number | null>(null);
  const [n6Redirect, setN6Redirect] = useState<number | null>(null);
  const [n3Aborted, setN3Aborted] = useState<number | null>(null);
  const [n6Aborted, setN6Aborted] = useState<number | null>(null);

  const [upSince, setUpSince] = useState<Date | null>(null);

  const [networkLogs, setRadioEvents] = useState<APIRadioEvent[]>([]);
  const [logsError, setLogsError] = useState<string | null>(null);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const parseMetrics = (metrics: string) => {
    const lines = metrics.split("\n");
    const getValue = (prefix: string): number | null => {
      const line = lines.find((l) => l.startsWith(prefix));
      if (!line) return null;
      const parts = line.split(" ");
      const n = Number(parts[1]);
      return Number.isFinite(n) ? n : null;
    };

    const pduSessions = getValue("app_pdu_sessions_total ");
    const heapBytes = getValue("go_memstats_heap_inuse_bytes ");
    const sysBytes = getValue("process_resident_memory_bytes ");
    const goGoroutines = getValue("go_goroutines ");
    const dbBytes = getValue("app_database_storage_bytes ");
    const allocIPs = getValue("app_ip_addresses_allocated_total ");
    const totalIPsV = getValue("app_ip_addresses_total ");
    const ulBytes = getValue("app_uplink_bytes ");
    const dlBytes = getValue("app_downlink_bytes ");
    const n3Drop = getValue(
      'app_xdp_action_total{action="XDP_DROP",interface="n3"} ',
    );
    const n6Drop = getValue(
      'app_xdp_action_total{action="XDP_DROP",interface="n6"} ',
    );
    const n3PassV = getValue(
      'app_xdp_action_total{action="XDP_PASS",interface="n3"} ',
    );
    const n6PassV = getValue(
      'app_xdp_action_total{action="XDP_PASS",interface="n6"} ',
    );
    const n3TxV = getValue(
      'app_xdp_action_total{action="XDP_TX",interface="n3"} ',
    );
    const n6TxV = getValue(
      'app_xdp_action_total{action="XDP_TX",interface="n6"} ',
    );
    const n3RedirectV = getValue(
      'app_xdp_action_total{action="XDP_REDIRECT",interface="n3"} ',
    );
    const n6RedirectV = getValue(
      'app_xdp_action_total{action="XDP_REDIRECT",interface="n6"} ',
    );
    const n3AbortedV = getValue(
      'app_xdp_action_total{action="XDP_ABORTED",interface="n3"} ',
    );
    const n6AbortedV = getValue(
      'app_xdp_action_total{action="XDP_ABORTED",interface="n6"} ',
    );
    const startTime = getValue("process_start_time_seconds ");

    return {
      pduSessions: pduSessions ?? null,
      heapMemoryBytes: heapBytes,
      totalMemoryBytes: sysBytes,
      databaseSizeBytes: dbBytes,
      routines: goGoroutines ?? null,
      allocatedIPs: allocIPs == null ? null : Math.round(allocIPs),
      totalIPs: totalIPsV == null ? null : Math.round(totalIPsV),
      uplinkBytes: ulBytes ?? null,
      downlinkBytes: dlBytes ?? null,
      n3Drops: n3Drop ?? null,
      n6Drops: n6Drop ?? null,
      n3Pass: n3PassV ?? null,
      n6Pass: n6PassV ?? null,
      n3Tx: n3TxV ?? null,
      n6Tx: n6TxV ?? null,
      n3Redirect: n3RedirectV ?? null,
      n6Redirect: n6RedirectV ?? null,
      n3Aborted: n3AbortedV ?? null,
      n6Aborted: n6AbortedV ?? null,
      processStart: startTime ?? null,
    };
  };

  useEffect(() => {
    if (!authReady) return;
    if (!accessToken) return;
    let mounted = true;

    (async () => {
      try {
        const [status, subsPage, radiosPage] = await Promise.all([
          getStatus(),
          listSubscribers(
            accessToken,
            1,
            1,
          ) as Promise<ListSubscribersResponse>,
          listRadios(accessToken, 1, 1),
        ]);
        if (!mounted) return;

        setVersion(status.version ?? null);
        setSubscriberCount(subsPage.total_count ?? 0);
        setRadioCount(radiosPage.total_count ?? 0);
      } catch {
        if (mounted) setError("Failed to fetch initial data.");
      }
    })();

    return () => {
      mounted = false;
    };
  }, [authReady, accessToken]);

  useEffect(() => {
    let interval: number | undefined;
    let mounted = true;

    const tick = async () => {
      try {
        const raw = await getMetrics();
        if (!mounted) return;

        const {
          pduSessions,
          heapMemoryBytes,
          totalMemoryBytes,
          databaseSizeBytes,
          routines,
          allocatedIPs,
          totalIPs,
          uplinkBytes,
          downlinkBytes,
          n3Drops,
          n6Drops,
          n3Pass,
          n6Pass,
          n3Tx,
          n6Tx,
          n3Redirect,
          n6Redirect,
          n3Aborted,
          n6Aborted,
          processStart,
        } = parseMetrics(raw);

        setActiveSessions(pduSessions);
        setHeapMemory(heapMemoryBytes);
        setTotalMemory(totalMemoryBytes);
        setDatabaseSize(databaseSizeBytes);
        setRoutines(routines);
        setAllocatedIPs(allocatedIPs);
        setTotalIPs(totalIPs);
        setUplinkBytes(uplinkBytes);
        setDownlinkBytes(downlinkBytes);
        setN3Drops(n3Drops);
        setN6Drops(n6Drops);
        setN3Pass(n3Pass);
        setN6Pass(n6Pass);
        setN3Tx(n3Tx);
        setN6Tx(n6Tx);
        setN3Redirect(n3Redirect);
        setN6Redirect(n6Redirect);
        setN3Aborted(n3Aborted);
        setN6Aborted(n6Aborted);

        if (processStart) {
          setUpSince(new Date(processStart * 1000));
        }

        setError(null);
      } catch (e) {
        console.error("Failed to update metrics:", e);
        setError((prev) => prev ?? "Failed to update metrics.");
      } finally {
        setLoading(false);
      }
    };

    const start = () => {
      tick();
      interval = window.setInterval(tick, 5000);
    };
    const stop = () => {
      if (interval) window.clearInterval(interval);
    };

    const handleVisibilityChange = () => {
      if (document.hidden) stop();
      else start();
    };

    start();
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      stop();
      mounted = false;
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, []);

  useEffect(() => {
    if (!authReady) return;
    if (!accessToken) return;

    let mounted = true;

    const fetchLogs = async () => {
      try {
        const res: ListRadioEventsResponse = await listRadioEvents(
          accessToken,
          1,
          10,
        );
        if (!mounted) return;
        setRadioEvents(res.items ?? []);
        setLogsError(null);
      } catch (e) {
        if (!mounted) return;
        console.error("Error fetching radio events:", e);
        setLogsError("Failed to fetch radio events.");
      }
    };

    fetchLogs();
    return () => {
      mounted = false;
    };
  }, [authReady, accessToken]);

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
                onClick={() => router.push("/radios")}
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
                onClick={() => router.push("/subscribers")}
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
                href="/radios?tab=events"
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
