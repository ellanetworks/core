import React, { useMemo, useState, useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Box, Typography, Button, TextField, MenuItem } from "@mui/material";
import { Link, useSearchParams } from "react-router-dom";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
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
  type AuditLogFilters,
} from "@/queries/audit_logs";
import { listUsers, type ListUsersResponse } from "@/queries/users";
import { useAuth } from "@/contexts/AuthContext";
import EditAuditLogRetentionPolicyModal from "@/components/EditAuditLogRetentionPolicyModal";
import { formatDateTime } from "@/utils/formatters";

const MAX_WIDTH = 1400;

const getDefaultDateRange = () => {
  const today = new Date();
  const sevenDaysAgo = new Date();
  sevenDaysAgo.setDate(today.getDate() - 6);
  const format = (d: Date) => d.toISOString().slice(0, 10);
  return { startDate: format(sevenDaysAgo), endDate: format(today) };
};

const AuditLog: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin";

  const outerTheme = useTheme();
  const gridTheme = useMemo(() => createTheme(outerTheme), [outerTheme]);

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const { showSnackbar } = useSnackbar();

  const [isEditModalOpen, setEditModalOpen] = useState(false);

  // ── Filters ─────────────────────────────────────────
  const [{ startDate, endDate }, setDateRange] = useState(getDefaultDateRange);
  const [searchParams] = useSearchParams();
  const [selectedActor, setSelectedActor] = useState(
    () => searchParams.get("actor") ?? "",
  );

  const handleStartChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) =>
      setDateRange((prev) => ({ ...prev, startDate: e.target.value })),
    [],
  );

  const handleEndChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) =>
      setDateRange((prev) => ({ ...prev, endDate: e.target.value })),
    [],
  );

  const descriptionText =
    "Review security-relevant actions performed in Ella Core. The audit log records who did what and when.";

  const queryClient = useQueryClient();
  const pageOneBased = paginationModel.page + 1;

  const { data: retentionPolicy } = useQuery<AuditLogRetentionPolicy>({
    queryKey: ["auditLogRetention"],
    queryFn: () => getAuditLogRetentionPolicy(accessToken || ""),
    enabled: authReady && !!accessToken,
  });

  // Fetch users for the actor filter dropdown
  const { data: usersData } = useQuery<ListUsersResponse>({
    queryKey: ["users", 1, 100],
    queryFn: () => listUsers(accessToken || "", 1, 100),
    enabled: authReady && !!accessToken,
  });

  const userOptions = useMemo(
    () => (usersData?.items ?? []).map((u) => u.email),
    [usersData],
  );

  // Build filters for the query
  const filters: AuditLogFilters = useMemo(() => {
    const f: AuditLogFilters = {};
    if (startDate) f.start = startDate;
    if (endDate) f.end = endDate;
    if (selectedActor) f.actor = selectedActor;
    return f;
  }, [startDate, endDate, selectedActor]);

  const { data: auditLogsData, isLoading: loading } =
    useQuery<ListAuditLogsResponse>({
      queryKey: ["auditLogs", pageOneBased, paginationModel.pageSize, filters],
      queryFn: () =>
        listAuditLogs(
          accessToken || "",
          pageOneBased,
          paginationModel.pageSize,
          filters,
        ),
      enabled: authReady && !!accessToken,
      placeholderData: (prev) => prev,
    });

  const rows: APIAuditLog[] = auditLogsData?.items ?? [];
  const rowCount = auditLogsData?.total_count ?? 0;

  const columns: GridColDef<APIAuditLog>[] = useMemo(
    () => [
      {
        field: "timestamp",
        headerName: "Timestamp",
        flex: 0,
        width: 130,
        sortable: false,
        valueFormatter: (value: string) => formatDateTime(value),
      },
      {
        field: "actor",
        headerName: "Actor",
        flex: 1,
        minWidth: 200,
        sortable: false,
        renderCell: (params) => {
          const actor = params.value as string;
          if (!actor) return null;
          return (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                width: "100%",
                height: "100%",
              }}
            >
              <Link
                to={`/users/${encodeURIComponent(actor)}`}
                style={{ textDecoration: "none" }}
                onClick={(e: React.MouseEvent) => e.stopPropagation()}
              >
                <Typography
                  variant="body2"
                  sx={{
                    color: "#4254FB",
                    textDecoration: "underline",
                    "&:hover": { textDecoration: "underline" },
                  }}
                >
                  {actor}
                </Typography>
              </Link>
            </Box>
          );
        },
      },
      {
        field: "action",
        headerName: "Action",
        flex: 1,
        minWidth: 180,
        sortable: false,
      },
      {
        field: "ip",
        headerName: "IP Address",
        flex: 1,
        minWidth: 130,
        sortable: false,
      },
      {
        field: "details",
        headerName: "Details",
        flex: 2,
        minWidth: 300,
        sortable: false,
        renderCell: (params) => (
          <Box
            sx={{
              display: "-webkit-box",
              WebkitLineClamp: 2,
              WebkitBoxOrient: "vertical",
              overflow: "hidden",
              whiteSpace: "normal",
              lineHeight: 1.4,
            }}
          >
            {params.value as string}
          </Box>
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

        {/* Filters row */}
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", sm: "row" },
            gap: 2,
            alignItems: { xs: "flex-start", sm: "center" },
          }}
        >
          <TextField
            label="Start date"
            type="date"
            value={startDate}
            onChange={handleStartChange}
            InputLabelProps={{ shrink: true }}
            size="small"
          />
          <TextField
            label="End date"
            type="date"
            value={endDate}
            onChange={handleEndChange}
            InputLabelProps={{ shrink: true }}
            size="small"
          />
          <TextField
            select
            label="User"
            value={selectedActor}
            onChange={(e) => setSelectedActor(e.target.value)}
            size="small"
            sx={{ minWidth: 200 }}
          >
            <MenuItem value="">All users</MenuItem>
            {userOptions.map((email) => (
              <MenuItem key={email} value={email}>
                {email}
              </MenuItem>
            ))}
          </TextField>
        </Box>

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
            paginationMode="server"
            rowCount={rowCount}
            paginationModel={paginationModel}
            onPaginationModelChange={setPaginationModel}
            sortingMode="server"
            disableColumnMenu
            disableRowSelectionOnClick
            pageSizeOptions={[10, 25, 50, 100]}
            rowHeight={52}
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
          queryClient.invalidateQueries({ queryKey: ["auditLogRetention"] });
          showSnackbar("Retention policy updated successfully.", "success");
        }}
        initialData={retentionPolicy || { days: 30 }}
      />
    </Box>
  );
};

export default AuditLog;
