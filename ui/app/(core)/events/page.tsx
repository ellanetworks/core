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
  Tabs,
  Tab,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  DataGrid,
  type GridColDef,
  type GridRenderCellParams,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import VisibilityIcon from "@mui/icons-material/Visibility";
import EastIcon from "@mui/icons-material/East";
import WestIcon from "@mui/icons-material/West";
import {
  listSubscriberLogs,
  clearSubscriberLogs,
  getSubscriberLogRetentionPolicy,
  type SubscriberLogRetentionPolicy,
  type APISubscriberLog,
  type ListSubscriberLogsResponse,
} from "@/queries/subscriber_logs";

import {
  listRadioLogs,
  clearRadioLogs,
  getRadioLogRetentionPolicy,
  type APIRadioLog,
  type ListRadioLogsResponse,
  type RadioLogRetentionPolicy,
} from "@/queries/radio_logs";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";

import { useAuth } from "@/contexts/AuthContext";
import EditSubscriberLogRetentionPolicyModal from "@/components/EditSubscriberLogRetentionPolicyModal";
import EditRadioLogRetentionPolicyModal from "@/components/EditRadioLogRetentionPolicyModal";
import ViewLogModal from "@/components/ViewLogModal";
import type { LogRow } from "@/components/ViewLogModal";

const MAX_WIDTH = 1400;
type TabKey = "subscribers" | "radio";

const DirectionCell: React.FC<{ value?: string }> = ({ value }) => {
  const theme = useTheme();

  if (!value) return null;

  const Icon = value === "inbound" ? EastIcon : WestIcon;
  const title = value === "inbound" ? "Receive (inbound)" : "Send (outbound)";
  const color =
    value === "inbound" ? theme.palette.success.main : theme.palette.info.main;

  return (
    <Tooltip title={title}>
      <Box
        sx={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          lineHeight: 0,
          "& svg": { display: "block" },
        }}
      >
        <Icon fontSize="small" sx={{ color }} aria-label={title} />
      </Box>
    </Tooltip>
  );
};

const Events: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin";

  const outerTheme = useTheme();
  const gridTheme = useMemo(() => createTheme(outerTheme), [outerTheme]);

  const [tab, setTab] = useState<TabKey>("subscribers");

  // ---------------- Shared UI ----------------
  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });
  const [viewLogModalOpen, setViewLogModalOpen] = useState(false);
  const [selectedRow, setSelectedRow] = useState<LogRow | null>(null);

  // ---------------- Subscribers tab state ----------------
  const [subRows, setSubRows] = useState<APISubscriberLog[]>([]);
  const [subRowCount, setSubRowCount] = useState<number>(0);
  const [subLoading, setSubLoading] = useState<boolean>(false);
  const [subPagination, setSubPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [isSubscriberEditModalOpen, setSubscriberEditModalOpen] =
    useState(false);
  const [isSubscriberClearModalOpen, setSubscriberClearModalOpen] =
    useState(false);
  const [isRadioEditModalOpen, setRadioEditModalOpen] = useState(false);
  const [subRetentionPolicy, setSubRetentionPolicy] =
    useState<SubscriberLogRetentionPolicy | null>(null);

  // ---------------- Radio tab state ----------------
  const [radioRows, setRadioRows] = useState<APIRadioLog[]>([]);
  const [radioRowCount, setRadioRowCount] = useState<number>(0);
  const [radioLoading, setRadioLoading] = useState<boolean>(false);
  const [radioPagination, setRadioPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });
  const [isRadioClearModalOpen, setRadioClearModalOpen] = useState(false);
  const [radioRetentionPolicy, setRadioRetentionPolicy] =
    useState<RadioLogRetentionPolicy | null>(null);

  // ---------------- Fetchers ----------------
  const fetchSubscriberRetention = useCallback(async () => {
    if (!authReady || !accessToken) return;
    try {
      const data = await getSubscriberLogRetentionPolicy(accessToken);
      setSubRetentionPolicy(data);
    } catch (e) {
      console.error("Error fetching subscriber log retention policy:", e);
    }
  }, [accessToken, authReady]);

  const fetchRadioRetention = useCallback(async () => {
    if (!authReady || !accessToken) return;
    try {
      const data = await getRadioLogRetentionPolicy(accessToken);
      setRadioRetentionPolicy(data);
    } catch (e) {
      console.error("Error fetching radio log retention policy:", e);
    }
  }, [accessToken, authReady]);

  const fetchSubscriberLogs = useCallback(
    async (pageZeroBased: number, pageSize: number) => {
      if (!authReady || !accessToken) return;
      setSubLoading(true);
      let mounted = true;
      try {
        const pageOneBased = pageZeroBased + 1;
        const data: ListSubscriberLogsResponse = await listSubscriberLogs(
          accessToken,
          pageOneBased,
          pageSize,
        );
        if (!mounted) return;
        setSubRows(data.items ?? []);
        setSubRowCount(data.total_count ?? 0);
      } catch (error) {
        console.error("Error fetching subscriber logs:", error);
      } finally {
        setSubLoading(false);
      }
      return () => {
        mounted = false;
      };
    },
    [accessToken, authReady],
  );

  const fetchRadioLogs = useCallback(
    async (pageZeroBased: number, pageSize: number) => {
      if (!authReady || !accessToken) return;
      setRadioLoading(true);
      let mounted = true;
      try {
        const pageOneBased = pageZeroBased + 1;
        const data: ListRadioLogsResponse = await listRadioLogs(
          accessToken,
          pageOneBased,
          pageSize,
        );
        if (!mounted) return;
        setRadioRows(data.items ?? []);
        setRadioRowCount(data.total_count ?? 0);
      } catch (error) {
        console.error("Error fetching radio logs:", error);
      } finally {
        setRadioLoading(false);
      }
      return () => {
        mounted = false;
      };
    },
    [accessToken, authReady],
  );

  const handleConfirmDeleteSubscriberLogs = async () => {
    setSubscriberClearModalOpen(false);
    if (!accessToken) return;
    try {
      await clearSubscriberLogs(accessToken);
      setAlert({
        message: `All subscriber logs cleared successfully!`,
        severity: "success",
      });
      fetchSubscriberLogs(subPagination.page, subPagination.pageSize);
    } catch (error: unknown) {
      setAlert({
        message: `Failed to clear subscriber logs: ${String(error)}`,
        severity: "error",
      });
    }
  };

  const handleConfirmDeleteRadioLogs = async () => {
    setRadioClearModalOpen(false);
    if (!accessToken) return;
    try {
      await clearRadioLogs(accessToken);
      setAlert({
        message: `All radio logs cleared successfully!`,
        severity: "success",
      });
      fetchRadioLogs(radioPagination.page, radioPagination.pageSize);
    } catch (error: unknown) {
      setAlert({
        message: `Failed to clear radio logs: ${String(error)}`,
        severity: "error",
      });
    }
  };

  // ---------------- Effects ----------------
  useEffect(() => {
    fetchSubscriberRetention();
    fetchRadioRetention();
  }, [fetchSubscriberRetention, fetchRadioRetention]);

  useEffect(() => {
    if (tab === "subscribers") {
      fetchSubscriberLogs(subPagination.page, subPagination.pageSize);
    }
  }, [tab, fetchSubscriberLogs, subPagination.page, subPagination.pageSize]);

  useEffect(() => {
    if (tab === "radio") {
      fetchRadioLogs(radioPagination.page, radioPagination.pageSize);
    }
  }, [tab, fetchRadioLogs, radioPagination.page, radioPagination.pageSize]);

  // ---------------- Columns ----------------
  const subscriberColumns: GridColDef<APISubscriberLog>[] = useMemo(
    () => [
      {
        field: "timestamp",
        headerName: "Timestamp",
        flex: 1,
        minWidth: 220,
        sortable: false,
      },
      {
        field: "imsi",
        headerName: "IMSI",
        flex: 1,
        minWidth: 220,
        sortable: false,
      },
      {
        field: "direction",
        headerName: "Dir",
        width: 70,
        align: "center",
        headerAlign: "center",
        sortable: false,
        filterable: false,
        renderCell: (params: GridRenderCellParams<APISubscriberLog>) => (
          <DirectionCell value={params.row.direction} />
        ),
      },
      {
        field: "event",
        headerName: "Event",
        flex: 1,
        minWidth: 200,
        sortable: false,
      },
      {
        field: "view",
        headerName: "",
        sortable: false,
        filterable: false,
        width: 60,
        align: "center",
        headerAlign: "center",
        renderCell: (params: GridRenderCellParams<APISubscriberLog>) => (
          <Tooltip title="View details">
            <IconButton
              color="primary"
              size="small"
              onClick={(e) => {
                e.stopPropagation();
                const r = params.row;
                setSelectedRow({
                  id: String(r.id),
                  timestamp: r.timestamp,
                  event_id: r.imsi,
                  event: r.event,
                  details: r.details ?? "",
                });
                setViewLogModalOpen(true);
              }}
              aria-label="View details"
            >
              <VisibilityIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        ),
      },
    ],
    [],
  );

  const radioColumns: GridColDef<APIRadioLog>[] = useMemo(
    () => [
      {
        field: "timestamp",
        headerName: "Timestamp",
        flex: 1,
        minWidth: 220,
        sortable: false,
      },
      {
        field: "ran_id",
        headerName: "RAN ID",
        flex: 1,
        minWidth: 180,
        sortable: false,
      },
      {
        field: "direction",
        headerName: "Dir",
        width: 70,
        align: "center",
        headerAlign: "center",
        sortable: false,
        filterable: false,
        renderCell: (params: GridRenderCellParams<APIRadioLog>) => (
          <DirectionCell value={params.row.direction} />
        ),
      },
      {
        field: "event",
        headerName: "Event",
        flex: 1,
        minWidth: 200,
        sortable: false,
      },
      {
        field: "view",
        headerName: "",
        sortable: false,
        filterable: false,
        width: 60,
        align: "center",
        headerAlign: "center",
        renderCell: (params: GridRenderCellParams<APIRadioLog>) => (
          <Tooltip title="View details">
            <IconButton
              color="primary"
              size="small"
              onClick={(e) => {
                e.stopPropagation();
                const r = params.row;
                setSelectedRow({
                  id: String(r.id),
                  timestamp: r.timestamp,
                  event_id: r.ran_id,
                  event: r.event,
                  details: r.details ?? "",
                });
                setViewLogModalOpen(true);
              }}
              aria-label="View details"
            >
              <VisibilityIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        ),
      },
    ],
    [],
  );

  // ---------------- Render ----------------
  const subDescription =
    "Review subscriber events in Ella Core. These logs are useful for auditing and troubleshooting purposes.";
  const radioDescription =
    "Review radio events in Ella Core. These logs are helpful for radio onboarding, session setup, and troubleshooting.";

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

        <Tabs
          value={tab}
          onChange={(_, v) => setTab(v as TabKey)}
          aria-label="Event tabs"
          sx={{ borderBottom: 1, borderColor: "divider", mb: 2 }}
        >
          <Tab value="subscribers" label="Subscribers" />
          <Tab value="radio" label="Radio" />
        </Tabs>
      </Box>

      {/* ---------------- Subscribers Tab ---------------- */}
      {tab === "subscribers" && (
        <>
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
              {subDescription}
            </Typography>

            <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
              {canEdit && (
                <>
                  <Button
                    variant="contained"
                    color="primary"
                    onClick={() => setSubscriberEditModalOpen(true)}
                    sx={{ minWidth: 140 }}
                  >
                    Edit Retention
                  </Button>
                  <Button
                    variant="contained"
                    color="error"
                    onClick={() => setSubscriberClearModalOpen(true)}
                    sx={{ minWidth: 140 }}
                  >
                    Clear All
                  </Button>
                </>
              )}
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{ ml: "auto" }}
              >
                Retention: <strong>{subRetentionPolicy?.days ?? "…"}</strong>{" "}
                days
              </Typography>
            </Box>
          </Box>

          <Box
            sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}
          >
            <ThemeProvider theme={gridTheme}>
              <DataGrid<APISubscriberLog>
                rows={subRows}
                columns={subscriberColumns}
                getRowId={(row) => row.id}
                loading={subLoading}
                paginationMode="server"
                rowCount={subRowCount}
                paginationModel={subPagination}
                onPaginationModelChange={setSubPagination}
                disableRowSelectionOnClick
                disableColumnMenu
                sortingMode="server"
                pageSizeOptions={[10, 25, 50, 100]}
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
        </>
      )}

      {/* ---------------- Radio Tab ---------------- */}
      {tab === "radio" && (
        <>
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
            <Typography variant="h4">Radio Events</Typography>
            <Typography variant="body1" color="text.secondary">
              {radioDescription}
            </Typography>

            <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
              {canEdit && (
                <>
                  <Button
                    variant="contained"
                    color="primary"
                    onClick={() => setRadioEditModalOpen(true)}
                    sx={{ minWidth: 140 }}
                  >
                    Edit Retention
                  </Button>
                  <Button
                    variant="contained"
                    color="error"
                    onClick={() => setRadioClearModalOpen(true)}
                    sx={{ minWidth: 140 }}
                  >
                    Clear All
                  </Button>
                </>
              )}
              <Typography
                variant="body2"
                color="text.secondary"
                sx={{ ml: "auto" }}
              >
                Retention: <strong>{radioRetentionPolicy?.days ?? "…"}</strong>{" "}
                days
              </Typography>
            </Box>
          </Box>

          <Box
            sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}
          >
            <ThemeProvider theme={gridTheme}>
              <DataGrid<APIRadioLog>
                rows={radioRows}
                columns={radioColumns}
                getRowId={(row) => row.id}
                loading={radioLoading}
                paginationMode="server"
                rowCount={radioRowCount}
                paginationModel={radioPagination}
                onPaginationModelChange={setRadioPagination}
                disableRowSelectionOnClick
                disableColumnMenu
                sortingMode="server"
                pageSizeOptions={[10, 25, 50, 100]}
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
        </>
      )}

      {/* ---------------- Modals ---------------- */}
      <ViewLogModal
        open={viewLogModalOpen}
        onClose={() => setViewLogModalOpen(false)}
        log={selectedRow}
      />

      <EditSubscriberLogRetentionPolicyModal
        open={isSubscriberEditModalOpen}
        onClose={() => setSubscriberEditModalOpen(false)}
        onSuccess={() => {
          fetchSubscriberRetention();
          setAlert({
            message: "Retention policy updated!",
            severity: "success",
          });
        }}
        initialData={subRetentionPolicy || { days: 30 }}
      />
      <EditRadioLogRetentionPolicyModal
        open={isRadioEditModalOpen}
        onClose={() => setRadioEditModalOpen(false)}
        onSuccess={() => {
          fetchRadioRetention();
          setAlert({
            message: "Retention policy updated!",
            severity: "success",
          });
        }}
        initialData={radioRetentionPolicy || { days: 30 }}
      />
      <DeleteConfirmationModal
        title="Clear All Subscriber Logs"
        description="Are you sure you want to clear all subscriber logs? This action cannot be undone."
        open={isSubscriberClearModalOpen}
        onClose={() => setSubscriberClearModalOpen(false)}
        onConfirm={handleConfirmDeleteSubscriberLogs}
      />
      <DeleteConfirmationModal
        title="Clear All Radio Logs"
        description="Are you sure you want to clear all radio logs? This action cannot be undone."
        open={isRadioClearModalOpen}
        onClose={() => setRadioClearModalOpen(false)}
        onConfirm={handleConfirmDeleteRadioLogs}
      />
    </Box>
  );
};

export default Events;
