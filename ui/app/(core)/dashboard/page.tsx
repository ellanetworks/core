"use client";
import React, { useState, useEffect, useRef } from "react";
import { Box, Typography, CircularProgress, Alert, Card } from "@mui/material";
import { getStatus } from "@/queries/status";
import { getMetrics } from "@/queries/metrics";
import { listSubscribers } from "@/queries/subscribers";
import { listRadios } from "@/queries/radios";
import { PieChart } from "@mui/x-charts/PieChart";
import Grid from "@mui/material/Grid2";
import { useCookies } from "react-cookie"

const Dashboard = () => {
  const [cookies, setCookie, removeCookie] = useCookies(['user_token']);

  const [version, setVersion] = useState<string | null>(null);
  const [subscriberCount, setSubscriberCount] = useState<number | null>(null);
  const [radioCount, setRadioCount] = useState<number | null>(null);
  const [activeSessions, setActiveSessions] = useState<number | null>(null);
  const [memoryUsage, setMemoryUsage] = useState<number | null>(null);
  const [databaseSize, setDatabaseSize] = useState<number | null>(null);
  const [allocatedIPs, setAllocatedIPs] = useState<number | null>(null);
  const [totalIPs, setTotalIPs] = useState<number | null>(null);
  const [uplinkThroughput, setUplinkThroughput] = useState<number | null>(null);
  const [downlinkThroughput, setDownlinkThroughput] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const prevUplinkBytes = useRef<number>(0);
  const prevDownlinkBytes = useRef<number>(0);
  const lastTimestamp = useRef<number>(Date.now());


  const parseMetrics = (metrics: string) => {
    const lines = metrics.split("\n");

    const pduSessionMetric = lines.find((line) =>
      line.startsWith("app_pdu_sessions ")
    );
    const memoryMetric = lines.find((line) =>
      line.startsWith("go_memstats_alloc_bytes ")
    );
    const databaseSizeMetric = lines.find((line) =>
      line.startsWith("app_database_storage_bytes ")
    );

    const allocatedIPsMetric = lines.find((line) =>
      line.startsWith("app_ip_addresses_allocated ")
    );
    const totalIPsMetric = lines.find((line) =>
      line.startsWith("app_ip_addresses_total ")
    );
    const uplinkMetric = lines.find((line) =>
      line.startsWith("app_uplink_bytes ")
    );
    const downlinkMetric = lines.find((line) =>
      line.startsWith("app_downlink_bytes ")
    );


    return {
      pduSessions: pduSessionMetric
        ? parseInt(pduSessionMetric.split(" ")[1], 10)
        : 0,
      memoryUsage: memoryMetric
        ? Math.round(parseFloat(memoryMetric.split(" ")[1]) / (1024 * 1024)) // Convert bytes to MB
        : 0,
      databaseSize: databaseSizeMetric
        ? Math.round(parseFloat(databaseSizeMetric.split(" ")[1]) / (1024)) // Convert bytes to KB
        : 0,
      allocatedIPs: allocatedIPsMetric
        ? parseInt(allocatedIPsMetric.split(" ")[1], 10)
        : 0,
      totalIPs: totalIPsMetric
        ? parseInt(totalIPsMetric.split(" ")[1], 10)
        : 0,
      uplinkBytes: uplinkMetric
        ? parseInt(uplinkMetric.split(" ")[1], 10)
        : 0,
      downlinkBytes: downlinkMetric
        ? parseInt(downlinkMetric.split(" ")[1], 10)
        : 0,
    };
  };

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [status, subscribers, radios, metrics] = await Promise.all([
          getStatus(),
          listSubscribers(cookies.user_token),
          listRadios(cookies.user_token),
          getMetrics(),
        ]);

        setVersion(status.version);
        setSubscriberCount(subscribers.length);
        setRadioCount(radios.length);

        const {
          pduSessions,
          memoryUsage,
          databaseSize,
          allocatedIPs,
          totalIPs,
          uplinkBytes,
          downlinkBytes,
        } = parseMetrics(metrics);

        setActiveSessions(pduSessions);
        setMemoryUsage(memoryUsage);
        setDatabaseSize(databaseSize);
        setAllocatedIPs(allocatedIPs);
        setTotalIPs(totalIPs);

        // Compute throughput
        const currentTime = Date.now();
        const elapsedTime = (currentTime - lastTimestamp.current) / 1000; // seconds

        if (elapsedTime > 0) {
          const uplinkRate = (uplinkBytes - prevUplinkBytes.current) / elapsedTime;
          const downlinkRate = (downlinkBytes - prevDownlinkBytes.current) / elapsedTime;

          setUplinkThroughput(uplinkRate);
          setDownlinkThroughput(downlinkRate);
        }

        // Update previous values
        prevUplinkBytes.current = uplinkBytes;
        prevDownlinkBytes.current = downlinkBytes;
        lastTimestamp.current = currentTime;
      } catch (err: any) {
        console.error("Failed to fetch data:", err);
        setError("Failed to fetch data.");
      } finally {
        setLoading(false);
      }
    };

    fetchData();
    const interval = setInterval(fetchData, 5000); // Refresh every 5 seconds

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
                      { id: 1, value: (totalIPs ?? 0) - (allocatedIPs ?? 0), label: "Available" },
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
                {uplinkThroughput !== null
                  ? `${(uplinkThroughput * 8 / 1_000_000).toFixed(2)} Mbps`
                  : "N/A"}
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
                {downlinkThroughput !== null
                  ? `${(downlinkThroughput * 8 / 1_000_000).toFixed(2)} Mbps`
                  : "N/A"}
              </Typography>
            )}
          </Card>
        </Grid>
      </Grid>

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