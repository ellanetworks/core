"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Box, Typography, Alert, Button, Collapse } from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import {
  DataGrid,
  type GridColDef,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import {
  listAuditLogs,
  getAuditLogRetentionPolicy,
  type APIAuditLog,
  type ListAuditLogsResponse,
  type AuditLogRetentionPolicy,
} from "@/queries/audit_logs";
import { useAuth } from "@/contexts/AuthContext";
import EditAuditLogRetentionPolicyModal from "@/components/EditAuditLogRetentionPolicyModal";

const MAX_WIDTH = 1400;

const AuditLog: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin";

  const outerTheme = useTheme();
  const gridTheme = useMemo(() => createTheme(outerTheme), [outerTheme]);
  const isSmDown = useMediaQuery(outerTheme.breakpoints.down("sm"));

  const [rows, setRows] = useState<APIAuditLog[]>([]);
  const [rowCount, setRowCount] = useState<number>(0);
  const [loading, setLoading] = useState<boolean>(false);
  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({
    message: "",
    severity: null,
  });

  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [retentionPolicy, setRetentionPolicy] =
    useState<AuditLogRetentionPolicy | null>(null);

  const descriptionText =
    "Review security-relevant actions performed in Ella Core. The audit log records who did what and when.";

  const fetchRetentionPolicy = useCallback(async () => {
    if (!authReady || !accessToken) return;
    try {
      const data = await getAuditLogRetentionPolicy(accessToken);
      setRetentionPolicy(data);
    } catch (error) {
      console.error("Error fetching audit log retention policy:", error);
    }
  }, [accessToken, authReady]);

  const fetchAuditLogs = useCallback(
    async (pageZeroBased: number, pageSize: number) => {
      if (!authReady || !accessToken) return;
      setLoading(true);
      try {
        const pageOneBased = pageZeroBased + 1;
        const data: ListAuditLogsResponse = await listAuditLogs(
          accessToken,
          pageOneBased,
          pageSize,
        );
        setRows(data.items);
        setRowCount(data.total_count ?? 0);
      } catch (error) {
        console.error("Error fetching audit logs:", error);
      } finally {
        setLoading(false);
      }
    },
    [accessToken, authReady],
  );

  useEffect(() => {
    fetchRetentionPolicy();
  }, [fetchRetentionPolicy]);

  useEffect(() => {
    fetchAuditLogs(paginationModel.page, paginationModel.pageSize);
  }, [fetchAuditLogs, paginationModel.page, paginationModel.pageSize]);

  const columns: GridColDef<APIAuditLog>[] = useMemo(
    () => [
      {
        field: "timestamp",
        headerName: "Timestamp",
        flex: 1,
        minWidth: 220,
        sortable: false,
      },
      {
        field: "actor",
        headerName: "Actor",
        flex: 1,
        minWidth: 250,
        sortable: false,
      },
      {
        field: "action",
        headerName: "Action",
        flex: 1,
        minWidth: 200,
        sortable: false,
      },
      {
        field: "ip",
        headerName: "IP Address",
        flex: 1,
        minWidth: 150,
        sortable: false,
      },
      {
        field: "details",
        headerName: "Details",
        flex: 2,
        minWidth: 350,
        sortable: false,
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
        <Typography variant="h4">Audit Logs</Typography>

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
          <DataGrid<APIAuditLog>
            rows={rows}
            columns={columns}
            getRowId={(row) => row.id}
            loading={loading}
            paginationMode="server"
            rowCount={rowCount}
            paginationModel={paginationModel}
            onPaginationModelChange={setPaginationModel}
            sortingMode="server"
            disableColumnMenu
            disableRowSelectionOnClick
            pageSizeOptions={[10, 25, 50, 100]}
            density={isSmDown ? "compact" : "standard"}
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

      <EditAuditLogRetentionPolicyModal
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

export default AuditLog;
