"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Box,
  Typography,
  Alert,
  Button,
  Collapse,
  IconButton,
  Tooltip,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import { DataGrid, GridColDef, GridRenderCellParams } from "@mui/x-data-grid";
import VisibilityIcon from "@mui/icons-material/Visibility";
import {
  listSubscriberLogs,
  getSubscriberLogRetentionPolicy,
} from "@/queries/subscriber_logs";
import { useCookies } from "react-cookie";
import { useAuth } from "@/contexts/AuthContext";
import EditSubscriberLogRetentionPolicyModal from "@/components/EditSubscriberLogRetentionPolicyModal";
import { SubscriberLogRetentionPolicy } from "@/types/types";
import ViewLogModal from "@/components/ViewSubscriberLogModal";

interface SubscriberLogData {
  id: string;
  timestamp: string;
  level: string;
  imsi: string;
  event: string;
  details: string;
}

const MAX_WIDTH = 1400;

const Events: React.FC = () => {
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
  const gridTheme = useMemo(() => createTheme(outerTheme), [outerTheme]);

  const [retentionPolicy, setRetentionPolicy] =
    useState<SubscriberLogRetentionPolicy | null>(null);

  const [viewLogModalOpen, setViewLogModalOpen] = useState(false);
  const [selectedRow, setSelectedRow] = useState<SubscriberLogData | null>(
    null,
  );

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

  const columns: GridColDef<SubscriberLogData>[] = useMemo(() => {
    return [
      { field: "timestamp", headerName: "Timestamp", flex: 1, minWidth: 220 },
      { field: "imsi", headerName: "IMSI", flex: 1, minWidth: 220 },
      { field: "event", headerName: "Event", flex: 1, minWidth: 200 },
      {
        field: "view",
        headerName: "",
        sortable: false,
        filterable: false,
        width: 60,
        align: "center",
        headerAlign: "center",
        renderCell: (params: GridRenderCellParams<SubscriberLogData>) => (
          <Tooltip title="View details">
            <IconButton
              color="primary"
              size="small"
              onClick={(e) => {
                e.stopPropagation();
                setSelectedRow(params.row);
                setViewLogModalOpen(true);
              }}
              aria-label="View details"
            >
              <VisibilityIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        ),
      },
    ];
  }, []);

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
          <DataGrid<SubscriberLogData>
            rows={subscriberLogs}
            showToolbar={true}
            columns={columns}
            getRowId={(row) => row.id}
            initialState={{
              sorting: { sortModel: [{ field: "timestamp", sort: "desc" }] },
              pagination: { paginationModel: { pageSize: 25, page: 0 } },
            }}
            pageSizeOptions={[10, 25, 50, 100]}
            disableRowSelectionOnClick
            columnVisibilityModel={{ id: !isSmDown }}
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
                backgroundColor: "#F5F5F5",
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

      <ViewLogModal
        open={viewLogModalOpen}
        onClose={() => setViewLogModalOpen(false)}
        log={selectedRow}
      />

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
