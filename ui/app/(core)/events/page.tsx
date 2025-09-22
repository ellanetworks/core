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
import {
  DataGrid,
  GridColDef,
  GridRenderCellParams,
  GridPaginationModel,
} from "@mui/x-data-grid";
import VisibilityIcon from "@mui/icons-material/Visibility";
import {
  listSubscriberLogs,
  getSubscriberLogRetentionPolicy,
  type APISubscriberLog,
  type ListSubscriberLogsResponse,
} from "@/queries/subscriber_logs";
import { useAuth } from "@/contexts/AuthContext";
import EditSubscriberLogRetentionPolicyModal from "@/components/EditSubscriberLogRetentionPolicyModal";
import { SubscriberLogRetentionPolicy } from "@/types/types";
import ViewLogModal from "@/components/ViewSubscriberLogModal";
import type { LogRow } from "@/components/ViewSubscriberLogModal";

const MAX_WIDTH = 1400;

const Events: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin";

  const outerTheme = useTheme();
  const gridTheme = useMemo(() => createTheme(outerTheme), [outerTheme]);

  const [rows, setRows] = useState<APISubscriberLog[]>([]);
  const [rowCount, setRowCount] = useState<number>(0);
  const [loading, setLoading] = useState<boolean>(false);

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });

  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [retentionPolicy, setRetentionPolicy] =
    useState<SubscriberLogRetentionPolicy | null>(null);

  const [viewLogModalOpen, setViewLogModalOpen] = useState(false);
  const [selectedRow, setSelectedRow] = useState<LogRow | null>(null);

  const descriptionText =
    "Review subscriber events in Ella Core. These logs are useful for auditing and troubleshooting purposes.";

  const fetchRetentionPolicy = useCallback(async () => {
    if (!authReady || !accessToken) return;
    try {
      const data = await getSubscriberLogRetentionPolicy(accessToken);
      setRetentionPolicy(data);
    } catch (error) {
      console.error("Error fetching subscriber log retention policy:", error);
    }
  }, [accessToken, authReady]);

  const fetchSubscriberLogs = useCallback(
    async (pageZeroBased: number, pageSize: number) => {
      if (!authReady || !accessToken) return;
      setLoading(true);
      let mounted = true;

      try {
        const pageOneBased = pageZeroBased + 1;
        const data: ListSubscriberLogsResponse = await listSubscriberLogs(
          accessToken,
          pageOneBased,
          pageSize,
        );
        if (!mounted) return;
        setRows(data.items);
        setRowCount(data.total_count ?? 0);
      } catch (error) {
        console.error("Error fetching subscriber logs:", error);
      } finally {
        setLoading(false);
      }

      return () => {
        mounted = false;
      };
    },
    [accessToken, authReady],
  );

  useEffect(() => {
    fetchRetentionPolicy();
  }, [fetchRetentionPolicy]);

  useEffect(() => {
    fetchSubscriberLogs(paginationModel.page, paginationModel.pageSize);
  }, [fetchSubscriberLogs, paginationModel.page, paginationModel.pageSize]);

  const columns: GridColDef<APISubscriberLog>[] = useMemo(
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
                  imsi: r.imsi,
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
          <DataGrid<APISubscriberLog>
            rows={rows}
            columns={columns}
            getRowId={(row) => row.id}
            loading={loading}
            paginationMode="server"
            rowCount={rowCount}
            paginationModel={paginationModel}
            onPaginationModelChange={setPaginationModel}
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
