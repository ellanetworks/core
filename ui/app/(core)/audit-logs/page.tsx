"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { Box, Typography, Alert, Button, Collapse } from "@mui/material";
import { useTheme } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import { DataGrid, GridColDef } from "@mui/x-data-grid";
import {
  listAuditLogs,
  deleteAuditLogs,
  getAuditLogRetentionPolicy,
} from "@/queries/audit_logs";
import { useCookies } from "react-cookie";
import { useAuth } from "@/contexts/AuthContext";
import EditAuditLogRetentionPolicyModal from "@/components/EditAuditLogRetentionPolicyModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { AuditLogRetentionPolicy } from "@/types/types";
import { ThemeProvider, createTheme } from "@mui/material/styles";

interface AuditLogData {
  id: string;
  timestamp: string;
  level: string;
  actor: string;
  action: string;
  ip: string;
  details: string;
}

const MAX_WIDTH = 1400;

const AuditLog = () => {
  const { role } = useAuth();
  const [cookies] = useCookies(["user_token"]);
  const [auditLogs, setAuditLogs] = useState<AuditLogData[]>([]);
  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({
    message: "",
    severity: null,
  });
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
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
    useState<AuditLogRetentionPolicy | null>(null);

  const descriptionText =
    "Review security-relevant actions performed in Ella Core. The audit log records who did what and when.";

  const fetchRetentionPolicy = useCallback(async () => {
    try {
      const data = await getAuditLogRetentionPolicy(cookies.user_token);
      setRetentionPolicy(data);
    } catch (error) {
      console.error("Error fetching audit log retention policy:", error);
    }
  }, [cookies.user_token]);

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    try {
      await deleteAuditLogs(cookies.user_token);
      setAlert({
        message: `Audit Logs deleted successfully!`,
        severity: "success",
      });
      fetchAuditLogs();
    } catch (error) {
      setAlert({
        message: `Failed to delete Audit Logs": ${
          error instanceof Error ? error.message : "Unknown error"
        }`,
        severity: "error",
      });
    }
  };

  const fetchAuditLogs = useCallback(async () => {
    try {
      const data = await listAuditLogs(cookies.user_token);
      setAuditLogs(data);
    } catch (error) {
      console.error("Error fetching audit logs:", error);
    }
  }, [cookies.user_token]);

  useEffect(() => {
    fetchAuditLogs();
    fetchRetentionPolicy();
  }, [fetchAuditLogs, fetchRetentionPolicy]);

  const columns: GridColDef[] = useMemo(
    () => [
      { field: "timestamp", headerName: "Timestamp", flex: 1, minWidth: 220 },
      { field: "actor", headerName: "Actor", flex: 1, minWidth: 250 },
      { field: "action", headerName: "Action", flex: 1, minWidth: 200 },
      { field: "ip", headerName: "IP Address", flex: 1, minWidth: 150 },
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
          <DataGrid
            rows={auditLogs}
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

      {isConfirmationOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setConfirmationOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete all audit logs? This action cannot be undone.`}
        />
      )}
    </Box>
  );
};

export default AuditLog;
