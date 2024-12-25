"use client";
import React, { useState, useEffect } from "react";
import { Box, Typography, CircularProgress, Alert } from "@mui/material";
import { getStatus } from "@/queries/status";
import { getMetrics } from "@/queries/metrics";
import { listSubscribers } from "@/queries/subscribers";
import Grid from "@mui/material/Grid2";

const Dashboard = () => {
  const [version, setVersion] = useState<string | null>(null);
  const [subscriberCount, setSubscriberCount] = useState<number | null>(null);
  const [activeSessions, setActiveSessions] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const parseMetrics = (metrics: string) => {
    const pduSessionMetric = metrics
      .split("\n")
      .find((line) => line.startsWith("pdu_sessions "));
    return pduSessionMetric ? parseInt(pduSessionMetric.split(" ")[1], 10) : 0;
  };

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [status, subscribers, metrics] = await Promise.all([
          getStatus(),
          listSubscribers(),
          getMetrics(),
        ]);

        setVersion(status.version);
        setSubscriberCount(subscribers.length);
        setActiveSessions(parseMetrics(metrics));
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

      <Grid container spacing={2} justifyContent="flex-start">
        <Grid size={{ xs: 6, sm: 3 }}>
          <Box
            sx={{
              width: "200px",
              height: "200px",
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
        <Grid size={{ xs: 6, sm: 3 }}>
          <Box
            sx={{
              width: "200px",
              height: "200px",
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
      </Grid>
    </Box>
  );
};

export default Dashboard;
