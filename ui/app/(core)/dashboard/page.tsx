"use client";

import React, { useEffect, useMemo, useRef, useState } from "react";
import {
  Box,
  Typography,
  CircularProgress,
  Alert,
  Card,
  CardActionArea,
  Skeleton,
} from "@mui/material";
import Grid from "@mui/material/Grid";
import { PieChart } from "@mui/x-charts/PieChart";
import { useCookies } from "react-cookie";
import { useRouter } from "next/navigation";

import { getStatus } from "@/queries/status";
import { getMetrics } from "@/queries/metrics";
import { listSubscribers } from "@/queries/subscribers";
import { listRadios } from "@/queries/radios";

const MAX_WIDTH = 1200;

const nf = new Intl.NumberFormat();
const formatNumber = (n: number | null | undefined) =>
  n == null ? "N/A" : nf.format(n);

const toMbps = (bytesPerSec: number) => (bytesPerSec * 8) / 1_000_000;
const formatMbps = (bps: number) => `${toMbps(bps).toFixed(2)} Mbps`;

const clampNonNegative = (n: number) => (n < 0 ? 0 : n);

type KpiCardProps = {
  title: string;
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
        p: 2,
      }}
    >
      <Typography variant="h6" sx={{ textAlign: "center", mb: 1 }}>
        {title}
      </Typography>
      <Box
        sx={{
          flex: 1,
          minHeight,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          textAlign: "center",
        }}
      >
        {body}
      </Box>
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
  const [cookies] = useCookies(["user_token"]);

  const [version, setVersion] = useState<string | null>(null);
  const [subscriberCount, setSubscriberCount] = useState<number | null>(null);
  const [radioCount, setRadioCount] = useState<number | null>(null);

  const [activeSessions, setActiveSessions] = useState<number | null>(null);
  const [memoryUsage, setMemoryUsage] = useState<number | null>(null);
  const [databaseSize, setDatabaseSize] = useState<number | null>(null);
  const [allocatedIPs, setAllocatedIPs] = useState<number | null>(null);
  const [totalIPs, setTotalIPs] = useState<number | null>(null);
  const [uplinkThroughput, setUplinkThroughput] = useState<number>(0);
  const [downlinkThroughput, setDownlinkThroughput] = useState<number>(0);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const uplinkHistory = useRef<number[]>([]);
  const downlinkHistory = useRef<number[]>([]);
  const timestamps = useRef<number[]>([]);

  const parseMetrics = (metrics: string) => {
    const lines = metrics.split("\n");

    const getValue = (prefix: string): number | null => {
      const line = lines.find((l) => l.startsWith(prefix));
      if (!line) return null;
      const parts = line.split(" ");
      const n = Number(parts[1]);
      return Number.isFinite(n) ? n : null;
    };

    const pduSessions = getValue("app_pdu_sessions ");
    const memBytes = getValue("go_memstats_alloc_bytes ");
    const dbBytes = getValue("app_database_storage_bytes ");
    const allocIPs = getValue("app_ip_addresses_allocated ");
    const totalIPs = getValue("app_ip_addresses_total ");
    const uplinkBytes = getValue("app_uplink_bytes ");
    const downlinkBytes = getValue("app_downlink_bytes ");

    return {
      pduSessions: pduSessions ?? null,
      memoryUsageMB:
        memBytes == null ? null : Math.round(memBytes / (1024 * 1024)),
      databaseSizeKB: dbBytes == null ? null : Math.round(dbBytes / 1024),
      allocatedIPs: allocIPs == null ? null : Math.round(allocIPs),
      totalIPs: totalIPs == null ? null : Math.round(totalIPs),
      uplinkBytes: uplinkBytes ?? 0,
      downlinkBytes: downlinkBytes ?? 0,
    };
  };

  useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        const [status, subscribers, radios] = await Promise.all([
          getStatus(),
          listSubscribers(cookies.user_token),
          listRadios(cookies.user_token),
        ]);
        if (!mounted) return;

        setVersion(status.version);
        setSubscriberCount(subscribers.length);
        setRadioCount(radios.length);
      } catch {
        if (mounted) setError("Failed to fetch initial data.");
      }
    })();
    return () => {
      mounted = false;
    };
  }, [cookies.user_token]);

  useEffect(() => {
    let interval: number | undefined;
    let mounted = true;

    const tick = async () => {
      try {
        const raw = await getMetrics();
        if (!mounted) return;

        const {
          pduSessions,
          memoryUsageMB,
          databaseSizeKB,
          allocatedIPs,
          totalIPs,
          uplinkBytes,
          downlinkBytes,
        } = parseMetrics(raw);

        setActiveSessions(pduSessions);
        setMemoryUsage(memoryUsageMB);
        setDatabaseSize(databaseSizeKB);
        setAllocatedIPs(allocatedIPs);
        setTotalIPs(totalIPs);

        const now = Date.now();
        uplinkHistory.current.push(uplinkBytes);
        downlinkHistory.current.push(downlinkBytes);
        timestamps.current.push(now);

        while (uplinkHistory.current.length > 5) {
          uplinkHistory.current.shift();
          downlinkHistory.current.shift();
          timestamps.current.shift();
        }

        if (uplinkHistory.current.length >= 2) {
          const deltaT =
            (timestamps.current[timestamps.current.length - 1] -
              timestamps.current[0]) /
            1000;
          if (deltaT > 0) {
            const uplinkDelta =
              uplinkHistory.current[uplinkHistory.current.length - 1] -
              uplinkHistory.current[0];
            const downlinkDelta =
              downlinkHistory.current[downlinkHistory.current.length - 1] -
              downlinkHistory.current[0];

            setUplinkThroughput(clampNonNegative(uplinkDelta / deltaT));
            setDownlinkThroughput(clampNonNegative(downlinkDelta / deltaT));
          }
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
      interval = window.setInterval(tick, 1000);
    };
    const stop = () => {
      if (interval) window.clearInterval(interval);
      interval = undefined;
    };

    const handleVisibility = () => {
      if (document.hidden) stop();
      else start();
    };

    start();
    document.addEventListener("visibilitychange", handleVisibility);
    return () => {
      mounted = false;
      stop();
      document.removeEventListener("visibilitychange", handleVisibility);
    };
  }, []);

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
          <KpiCard
            title="Radios"
            loading={loading}
            value={formatNumber(radioCount)}
            onClick={() => router.push("/radios")}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <KpiCard
            title="Subscribers"
            loading={loading}
            value={formatNumber(subscriberCount)}
            onClick={() => router.push("/subscribers")}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <KpiCard
            title="Active Sessions"
            loading={loading}
            value={formatNumber(activeSessions)}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 12, md: 6 }}>
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
      </Grid>

      <Grid
        container
        spacing={4}
        alignItems="stretch"
        justifyContent="flex-start"
        sx={{ mt: 2 }}
      >
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <KpiCard
            title="Uplink Throughput"
            loading={loading}
            value={formatMbps(uplinkThroughput)}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <KpiCard
            title="Downlink Throughput"
            loading={loading}
            value={formatMbps(downlinkThroughput)}
          />
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
          <KpiCard
            title="Memory Usage"
            loading={loading}
            value={
              memoryUsage != null ? `${formatNumber(memoryUsage)} MB` : "N/A"
            }
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <KpiCard
            title="Database Size"
            loading={loading}
            value={
              databaseSize != null ? `${formatNumber(databaseSize)} KB` : "N/A"
            }
          />
        </Grid>
      </Grid>
    </Box>
  );
};

export default Dashboard;
