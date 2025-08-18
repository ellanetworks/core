"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Box, Typography, Alert, Button, Collapse } from "@mui/material";
import { useTheme } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import { DataGrid, GridColDef } from "@mui/x-data-grid";
import {
  listSubscriberLogs,
  getSubscriberLogRetentionPolicy,
} from "@/queries/subscriber_logs";
import { useCookies } from "react-cookie";
import { useAuth } from "@/contexts/AuthContext";
import EditSubscriberLogRetentionPolicyModal from "@/components/EditSubscriberLogRetentionPolicyModal";
import { SubscriberLogRetentionPolicy } from "@/types/types";
import { ThemeProvider, createTheme } from "@mui/material/styles";

interface SubscriberLogData {
  id: string;
  timestamp: string;
  level: string;
  imsi: string;
  event: string;
  ip: string;
  details: string;
}

const MAX_WIDTH = 1400;

const Events = () => {
  const { role } = useAuth();
  const [cookies] = useCookies(["user_token"]);
  const [subscriberLogs, setSubscriberLogs] = useState<SubscriberLogData[]>([]);
  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({
    message: "",
    severity: null,
  });
  const [isEditModalOpen, setEditModalOpen] = useState(false);

  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));
  const canEdit = role === "Admin";

  const outerTheme = useTheme();
  const gridTheme = React.useMemo(
    () =>
      createTheme(outerTheme, {
        palette: {
          DataGrid: { headerBg: "#F5F5F5" },
        },
      }),
    [outerTheme],
  );

  const [retentionPolicy, setRetentionPolicy] =
    useState<SubscriberLogRetentionPolicy | null>(null);

  const descriptionText =
    "Review subscriber events in Ella Core. These logs are useful for auditing and troubleshooting purposes.";

  const fetchRetentionPolicy = useCallback(async () => {
    try {
      const data = await getSubscriberLogRetentionPolicy(cookies.user_token);
      setRetentionPolicy(data);
    } catch (error) {
      console.error("Error fetching subscriber log retention policy:", error);
    }
  }, [cookies.user_token]);

  const fetchSubscriberLogs = useCallback(async () => {
    try {
      const data = await listSubscriberLogs(cookies.user_token);
      setSubscriberLogs(data);
    } catch (error) {
      console.error("Error fetching subscriber logs:", error);
    }
  }, [cookies.user_token]);

  useEffect(() => {
    fetchSubscriberLogs();
    fetchRetentionPolicy();
  }, [fetchSubscriberLogs, fetchRetentionPolicy]);

  const columns: GridColDef[] = useMemo(
    () => [
      { field: "timestamp", headerName: "Timestamp", flex: 1, minWidth: 220 },
      { field: "imsi", headerName: "IMSI", flex: 1, minWidth: 250 },
      { field: "event", headerName: "Event", flex: 1, minWidth: 200 },
      { field: "details", headerName: "Details", flex: 2, minWidth: 350 },
    ],
    [],
  );

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        pt: 6,
        pb: 4,
      }}
    >
      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}>
        <Collapse in={!!alert.message}>
          <Alert
            severity={alert.severity || "success"}
            onClose={() => setAlert({ message: "", severity: null })}
            sx={{ mb: 2 }}
          >
            {alert.message}
          </Alert>
        </Collapse>
      </Box>

      <Box
        sx={{
          width: "100%",
          maxWidth: MAX_WIDTH,
          px: { xs: 2, sm: 4 },
          mb: 3,
          display: "flex",
          flexDirection: "column",
          gap: 2,
        }}
      >
        <Typography variant="h4">Subscriber Events</Typography>

        <Typography variant="body1" color="text.secondary">
          {descriptionText}
        </Typography>

        <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
          {canEdit && (
            <Button
              variant="contained"
              color="primary"
              onClick={() => setEditModalOpen(true)}
              sx={{ minWidth: 140 }}
            >
              Edit Retention
            </Button>
          )}

          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ ml: "auto" }}
          >
            Retention: <strong>{retentionPolicy?.days ?? "…"}</strong> days
          </Typography>
        </Box>
      </Box>

      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}>
        <ThemeProvider theme={gridTheme}>
          <DataGrid
            rows={subscriberLogs}
            columns={columns}
            getRowId={(row) => row.id}
            showToolbar={true}
            initialState={{
              sorting: { sortModel: [{ field: "timestamp", sort: "desc" }] },
            }}
            disableRowSelectionOnClick
            columnVisibilityModel={{
              id: !isSmDown,
            }}
            sx={{
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
              },
              "& .MuiDataGrid-footerContainer": {
                borderTop: "1px solid",
                borderColor: "divider",
              },
              "& .MuiDataGrid-columnHeaderTitle": { fontWeight: "bold" },
            }}
          />
        </ThemeProvider>
      </Box>

      <EditSubscriberLogRetentionPolicyModal
        open={isEditModalOpen}
        onClose={() => setEditModalOpen(false)}
        onSuccess={() => {
          fetchRetentionPolicy();
          setAlert({
            message: "Retention policy updated!",
            severity: "success",
          });
        }}
        initialData={retentionPolicy || { days: 30 }}
      />
    </Box>
  );
};

export default Events;
