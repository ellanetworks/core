"use client";

import React, { useState, useEffect, useRef } from "react";
import { Box, Typography, CircularProgress, Alert, Card } from "@mui/material";
import { getStatus } from "@/queries/status";
import { getMetrics } from "@/queries/metrics";
import { listSubscribers } from "@/queries/subscribers";
import { listRadios } from "@/queries/radios";
import { PieChart } from "@mui/x-charts/PieChart";
import Grid from "@mui/material/Grid";
import { useCookies } from "react-cookie";

const Dashboard = () => {
  const [cookies] = useCookies(["user_token"]);

  // Static values – update these only once on mount.
  const [version, setVersion] = useState<string | null>(null);
  const [subscriberCount, setSubscriberCount] = useState<number | null>(null);
  const [radioCount, setRadioCount] = useState<number | null>(null);

  // Metrics state – refreshed every second.
  const [activeSessions, setActiveSessions] = useState<number | null>(null);
  const [memoryUsage, setMemoryUsage] = useState<number | null>(null);
  const [databaseSize, setDatabaseSize] = useState<number | null>(null);
  const [allocatedIPs, setAllocatedIPs] = useState<number | null>(null);
  const [totalIPs, setTotalIPs] = useState<number | null>(null);
  const [uplinkThroughput, setUplinkThroughput] = useState<number>(0);
  const [downlinkThroughput, setDownlinkThroughput] = useState<number>(0);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Refs for throughput history calculation
  const uplinkHistory = useRef<number[]>([]);
  const downlinkHistory = useRef<number[]>([]);
  const timestamps = useRef<number[]>([]);

  // Parses the metrics string and returns all relevant values.
  const parseMetrics = (metrics: string) => {
    const lines = metrics.split("\n");

    const pduSessionMetric = lines.find((line) =>
      line.startsWith("app_pdu_sessions "),
    );
    const memoryMetric = lines.find((line) =>
      line.startsWith("go_memstats_alloc_bytes "),
    );
    const databaseSizeMetric = lines.find((line) =>
      line.startsWith("app_database_storage_bytes "),
    );
    const allocatedIPsMetric = lines.find((line) =>
      line.startsWith("app_ip_addresses_allocated "),
    );
    const totalIPsMetric = lines.find((line) =>
      line.startsWith("app_ip_addresses_total "),
    );
    const uplinkMetric = lines.find((line) =>
      line.startsWith("app_uplink_bytes "),
    );
    const downlinkMetric = lines.find((line) =>
      line.startsWith("app_downlink_bytes "),
    );

    return {
      pduSessions: pduSessionMetric
        ? parseInt(pduSessionMetric.split(" ")[1], 10)
        : 0,
      memoryUsage: memoryMetric
        ? Math.round(parseFloat(memoryMetric.split(" ")[1]) / (1024 * 1024))
        : 0,
      databaseSize: databaseSizeMetric
        ? Math.round(parseFloat(databaseSizeMetric.split(" ")[1]) / 1024)
        : 0,
      allocatedIPs: allocatedIPsMetric
        ? parseInt(allocatedIPsMetric.split(" ")[1], 10)
        : 0,
      totalIPs: totalIPsMetric ? parseInt(totalIPsMetric.split(" ")[1], 10) : 0,
      uplinkBytes: uplinkMetric ? parseFloat(uplinkMetric.split(" ")[1]) : 0,
      downlinkBytes: downlinkMetric
        ? parseFloat(downlinkMetric.split(" ")[1])
        : 0,
    };
  };

  // Static fetch for status, subscribers, and radios.
  useEffect(() => {
    const fetchInitialData = async () => {
      try {
        const [status, subscribers, radios] = await Promise.all([
          getStatus(),
          listSubscribers(cookies.user_token),
          listRadios(cookies.user_token),
        ]);

        setVersion(status.version);
        setSubscriberCount(subscribers.length);
        setRadioCount(radios.length);
      } catch (err: any) {
        console.error("Failed to fetch initial data:", err);
        setError("Failed to fetch initial data.");
      }
    };

    fetchInitialData();
  }, [cookies.user_token]);

  // Metrics update every second: this call updates all metrics state and throughput.
  useEffect(() => {
    const updateMetrics = async () => {
      try {
        const metrics = await getMetrics();
        const {
          pduSessions,
          memoryUsage,
          databaseSize,
          allocatedIPs,
          totalIPs,
          uplinkBytes,
          downlinkBytes,
        } = parseMetrics(metrics);

        // Update static metric states.
        setActiveSessions(pduSessions);
        setMemoryUsage(memoryUsage);
        setDatabaseSize(databaseSize);
        setAllocatedIPs(allocatedIPs);
        setTotalIPs(totalIPs);

        // Throughput history: add new data point.
        const currentTime = Date.now();
        uplinkHistory.current.push(uplinkBytes);
        downlinkHistory.current.push(downlinkBytes);
        timestamps.current.push(currentTime);

        // Keep only the last 5 samples.
        if (uplinkHistory.current.length > 5) {
          uplinkHistory.current.shift();
          downlinkHistory.current.shift();
          timestamps.current.shift();
        }

        // Calculate throughput if enough samples have been collected.
        if (uplinkHistory.current.length === 5) {
          const timeDelta =
            (timestamps.current[timestamps.current.length - 1] -
              timestamps.current[0]) /
            1000; // in seconds
          if (timeDelta > 0) {
            const uplinkRate =
              (uplinkHistory.current[uplinkHistory.current.length - 1] -
                uplinkHistory.current[0]) /
              timeDelta;
            const downlinkRate =
              (downlinkHistory.current[downlinkHistory.current.length - 1] -
                downlinkHistory.current[0]) /
              timeDelta;

            setUplinkThroughput(uplinkRate);
            setDownlinkThroughput(downlinkRate);
          }
        }
      } catch (err) {
        console.error("Failed to update metrics:", err);
        setError("Failed to update metrics.");
      } finally {
        setLoading(false);
      }
    };

    // Call immediately then every 1 second.
    updateMetrics();
    const interval = setInterval(updateMetrics, 1000);
    return () => clearInterval(interval);
  }, []);

  return (
    <Box sx={{ padding: 4, maxWidth: "1200px", margin: "0 auto" }}>
      <Typography
        variant="h4"
        component="h1"
        gutterBottom
        sx={{ textAlign: "left", marginBottom: 4 }}
      >
        Ella Core {loading ? <CircularProgress size={24} /> : version}
      </Typography>

      {error && (
        <Alert severity="error" sx={{ marginBottom: 4 }}>
          {error}
        </Alert>
      )}

      {/* Network Section */}
      <Typography
        variant="h5"
        component="h2"
        gutterBottom
        sx={{ textAlign: "left", marginTop: 4 }}
      >
        Network
      </Typography>
      <Grid container spacing={4} justifyContent="flex-start">
        <Grid size={3}>
          <Card
            sx={{
              width: "100%",
              aspectRatio: "1 / 1",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              borderRadius: 3,
              boxShadow: 2,
              padding: 2,
              backgroundColor: "background.paper",
              textAlign: "center",
            }}
          >
            <Typography variant="h6">Radios</Typography>
            {loading ? (
              <CircularProgress />
            ) : (
              <Typography variant="h4">{radioCount ?? 0}</Typography>
            )}
          </Card>
        </Grid>
        <Grid size={3}>
          <Card
            sx={{
              width: "100%",
              aspectRatio: "1 / 1",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              borderRadius: 3,
              boxShadow: 2,
              padding: 2,
              backgroundColor: "background.paper",
              textAlign: "center",
            }}
          >
            <Typography variant="h6">Subscribers</Typography>
            {loading ? (
              <CircularProgress />
            ) : (
              <Typography variant="h4">{subscriberCount ?? 0}</Typography>
            )}
          </Card>
        </Grid>
        <Grid size={3}>
          <Card
            sx={{
              width: "100%",
              aspectRatio: "1 / 1",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              borderRadius: 3,
              boxShadow: 2,
              padding: 2,
              backgroundColor: "background.paper",
              textAlign: "center",
            }}
          >
            <Typography variant="h6">Active Sessions</Typography>
            {loading ? (
              <CircularProgress />
            ) : (
              <Typography variant="h4">{activeSessions ?? 0}</Typography>
            )}
          </Card>
        </Grid>
        <Grid size={6}>
          <Card
            sx={{
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              borderRadius: 3,
              boxShadow: 2,
              padding: 2,
              backgroundColor: "background.paper",
              textAlign: "center",
            }}
          >
            <Typography variant="h6">IP Allocation</Typography>
            {loading ? (
              <CircularProgress />
            ) : (
              <PieChart
                series={[
                  {
                    data: [
                      { id: 0, value: allocatedIPs ?? 0, label: "Allocated" },
                      {
                        id: 1,
                        value: (totalIPs ?? 0) - (allocatedIPs ?? 0),
                        label: "Available",
                      },
                    ],
                  },
                ]}
                width={400}
                height={200}
              />
            )}
          </Card>
        </Grid>
      </Grid>

      {/* Throughput Section */}
      <Grid container spacing={4} justifyContent="flex-start" marginTop={4}>
        <Grid size={3}>
          <Card
            sx={{
              width: "100%",
              aspectRatio: "1 / 1",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              borderRadius: 3,
              boxShadow: 2,
              padding: 2,
              backgroundColor: "background.paper",
              textAlign: "center",
            }}
          >
            <Typography variant="h6">Uplink Throughput</Typography>
            {loading ? (
              <CircularProgress />
            ) : (
              <Typography variant="h4">
                {`${((uplinkThroughput * 8) / 1_000_000).toFixed(2)} Mbps`}
              </Typography>
            )}
          </Card>
        </Grid>
        <Grid size={3}>
          <Card
            sx={{
              width: "100%",
              aspectRatio: "1 / 1",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              borderRadius: 3,
              boxShadow: 2,
              padding: 2,
              backgroundColor: "background.paper",
              textAlign: "center",
            }}
          >
            <Typography variant="h6">Downlink Throughput</Typography>
            {loading ? (
              <CircularProgress />
            ) : (
              <Typography variant="h4">
                {`${((downlinkThroughput * 8) / 1_000_000).toFixed(2)} Mbps`}
              </Typography>
            )}
          </Card>
        </Grid>
      </Grid>

      {/* System Section */}
      <Typography
        variant="h5"
        component="h2"
        gutterBottom
        sx={{ textAlign: "left", marginTop: 4 }}
      >
        System
      </Typography>
      <Grid container spacing={2} justifyContent="flex-start">
        <Grid size={3}>
          <Card
            sx={{
              width: "100%",
              aspectRatio: "1 / 1",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              borderRadius: 3,
              boxShadow: 2,
              padding: 2,
              backgroundColor: "background.paper",
              textAlign: "center",
            }}
          >
            <Typography variant="h6">Memory Usage</Typography>
            {loading ? (
              <CircularProgress />
            ) : (
              <Typography variant="h4">
                {memoryUsage !== null ? `${memoryUsage} MB` : "N/A"}
              </Typography>
            )}
          </Card>
        </Grid>
        <Grid size={3}>
          <Card
            sx={{
              width: "100%",
              aspectRatio: "1 / 1",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              borderRadius: 3,
              boxShadow: 2,
              padding: 2,
              backgroundColor: "background.paper",
              textAlign: "center",
            }}
          >
            <Typography variant="h6">Database Size</Typography>
            {loading ? (
              <CircularProgress />
            ) : (
              <Typography variant="h4">
                {databaseSize !== null ? `${databaseSize} KB` : "N/A"}
              </Typography>
            )}
          </Card>
        </Grid>
      </Grid>
    </Box>
  );
};

export default Dashboard;
