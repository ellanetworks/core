"use client";
import React, { useState, useEffect } from "react";
import { Box, Typography, CircularProgress, Alert } from "@mui/material";
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
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const parseMetrics = (metrics: string) => {
    const lines = metrics.split("\n");

    const pduSessionMetric = lines.find((line) =>
      line.startsWith("pdu_sessions ")
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

        const { pduSessions, memoryUsage, databaseSize, allocatedIPs, totalIPs } = parseMetrics(metrics);
        setActiveSessions(pduSessions);
        setMemoryUsage(memoryUsage);
        setDatabaseSize(databaseSize);
        setAllocatedIPs(allocatedIPs);
        setTotalIPs(totalIPs);
        console.log("Allocated IPs: ", allocatedIPs);
        console.log("Total IPs: ", totalIPs);
      } catch (err: any) {
        console.error("Failed to fetch data:", err);
        setError("Failed to fetch data.");
      } finally {
        setLoading(false);
      }
    };

    fetchData();
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
          <Box
            sx={{
              width: "100%",
              aspectRatio: "1 / 1",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              border: "1px solid",
              borderColor: "divider",
              borderRadius: 2,
              textAlign: "center",
            }}
          >
            <Typography variant="h6">Radios</Typography>
            {loading ? (
              <CircularProgress />
            ) : (
              <Typography variant="h4">{radioCount ?? 0}</Typography>
            )}
          </Box>
        </Grid>
        <Grid size={3}>
          <Box
            sx={{
              width: "100%",
              aspectRatio: "1 / 1",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              border: "1px solid",
              borderColor: "divider",
              borderRadius: 2,
              textAlign: "center",
            }}
          >
            <Typography variant="h6">Subscribers</Typography>
            {loading ? (
              <CircularProgress />
            ) : (
              <Typography variant="h4">{subscriberCount ?? 0}</Typography>
            )}
          </Box>
        </Grid>
        <Grid size={3}>
          <Box
            sx={{
              width: "100%",
              aspectRatio: "1 / 1",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              border: "1px solid",
              borderColor: "divider",
              borderRadius: 2,
              textAlign: "center",
            }}
          >
            <Typography variant="h6">Active Sessions</Typography>
            {loading ? (
              <CircularProgress />
            ) : (
              <Typography variant="h4">{activeSessions ?? 0}</Typography>
            )}
          </Box>
        </Grid>
        <Grid size={6}>
          <Box
            sx={{
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              border: "1px solid",
              borderColor: "divider",
              borderRadius: 2,
              textAlign: "center",
              padding: 2,
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
          </Box>
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
          <Box
            sx={{
              width: "100%",
              aspectRatio: "1 / 1",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              border: "1px solid",
              borderColor: "divider",
              borderRadius: 2,
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
          </Box>
        </Grid>
        <Grid size={3}>
          <Box
            sx={{
              width: "100%",
              aspectRatio: "1 / 1",
              display: "flex",
              flexDirection: "column",
              justifyContent: "center",
              alignItems: "center",
              border: "1px solid",
              borderColor: "divider",
              borderRadius: 2,
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
          </Box>
        </Grid>
      </Grid>
    </Box>
  );
};

export default Dashboard;