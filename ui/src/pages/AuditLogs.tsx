// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useState, useCallback } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Box,
  Typography,
  TextField,
  MenuItem,
  IconButton,
  Tooltip,
} from "@mui/material";
import EditIcon from "@mui/icons-material/Edit";
import { Link, useSearchParams } from "react-router-dom";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useTheme } from "@mui/material/styles";
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
import QueryState from "@/components/QueryState";
import EmptyState from "@/components/EmptyState";
import EditAuditLogRetentionPolicyModal from "@/components/EditAuditLogRetentionPolicyModal";
import { formatDateTime } from "@/utils/formatters";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

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
  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const { showSnackbar } = useSnackbar();

  const [isEditModalOpen, setEditModalOpen] = useState(false);

  const [{ startDate, endDate }, setDateRange] = useState(getDefaultDateRange);
  const [searchParams] = useSearchParams();
  const [selectedUser, setSelectedUser] = useState(
    () => searchParams.get("user") ?? "",
  );
  const [selectedAction, setSelectedAction] = useState(
    () => searchParams.get("action") ?? "",
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

  const { data: usersData } = useQuery<ListUsersResponse>({
    queryKey: ["users", 1, 100],
    queryFn: () => listUsers(accessToken || "", 1, 100),
    enabled: authReady && !!accessToken,
  });

  const userOptions = useMemo(
    () => (usersData?.items ?? []).map((u) => u.email),
    [usersData],
  );

  const filters: AuditLogFilters = useMemo(() => {
    const f: AuditLogFilters = {};
    if (startDate) f.start = startDate;
    if (endDate) f.end = endDate;
    if (selectedUser) f.user = selectedUser;
    if (selectedAction) f.action = selectedAction;
    return f;
  }, [startDate, endDate, selectedUser, selectedAction]);

  const auditLogsQuery = useQuery<ListAuditLogsResponse>({
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

  const hasActiveFilters = Boolean(
    startDate || endDate || selectedUser || selectedAction,
  );

  const dateError =
    startDate && endDate && startDate > endDate
      ? "End date must be after start date"
      : "";

  const rowCount = auditLogsQuery.data?.total_count ?? 0;

  const columns: GridColDef<APIAuditLog>[] = useMemo(
    () => [
      {
        field: "timestamp",
        headerName: "Timestamp",
        flex: 0,
        width: 180,
        sortable: false,
        valueFormatter: (value: string) =>
          formatDateTime(value, { seconds: true }),
      },
      {
        field: "user",
        headerName: "User",
        flex: 1,
        minWidth: 120,
        sortable: false,
        renderCell: (params) => {
          const user = params.value as string;
          if (!user) return null;
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
                to={`/users/${encodeURIComponent(user)}`}
                style={{ textDecoration: "none" }}
                onClick={(e: React.MouseEvent) => e.stopPropagation()}
              >
                <Typography
                  variant="body2"
                  sx={{
                    color: outerTheme.palette.link,
                    textDecoration: "underline",
                    "&:hover": { textDecoration: "underline" },
                  }}
                >
                  {user}
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
        minWidth: 120,
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
        minWidth: 150,
        sortable: false,
        renderCell: (params) => {
          const text = params.value as string;
          return (
            <Tooltip title={text || ""} enterDelay={500} placement="top-start">
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
                {text}
              </Box>
            </Tooltip>
          );
        },
      },
    ],
    [outerTheme],
  );

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <Box
        sx={{
          mb: 3,
          display: "flex",
          flexDirection: "column",
          gap: 2,
        }}
      >
        <Typography variant="h4">Audit Logs</Typography>

        <Typography variant="body1" color="textSecondary">
          {descriptionText}
        </Typography>

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
            slotProps={{ inputLabel: { shrink: true } }}
            size="small"
          />
          <TextField
            label="End date"
            type="date"
            value={endDate}
            onChange={handleEndChange}
            size="small"
            error={!!dateError}
            helperText={dateError}
            slotProps={{
              inputLabel: { shrink: true },
              input: { inputProps: { min: startDate || undefined } },
            }}
          />
          <TextField
            select
            label="User"
            value={selectedUser}
            onChange={(e) => setSelectedUser(e.target.value)}
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
          <TextField
            label="Action"
            value={selectedAction}
            onChange={(e) => setSelectedAction(e.target.value)}
            size="small"
            sx={{ minWidth: 200 }}
            placeholder="e.g. create_subscriber"
          />

          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1,
              ml: { sm: "auto" },
            }}
          >
            <Typography variant="body2" color="textSecondary">
              Retention: <strong>{retentionPolicy?.days ?? "…"}</strong> days
            </Typography>
            {canEdit && (
              <IconButton
                aria-label="edit audit log retention"
                size="small"
                color="primary"
                onClick={() => setEditModalOpen(true)}
              >
                <EditIcon fontSize="small" />
              </IconButton>
            )}
          </Box>
        </Box>
      </Box>

      <QueryState
        query={auditLogsQuery}
        resource="audit logs"
        isEmpty={(data) => (data.total_count ?? 0) === 0}
        filtered={hasActiveFilters}
        noResults={
          <EmptyState
            primaryText="No audit logs match the selected filters"
            secondaryText="Try widening the date range or clearing the user and action filters."
          />
        }
        empty={
          <EmptyState
            primaryText="No audit logs yet"
            secondaryText="Actions taken in Ella Core will be recorded here."
          />
        }
      >
        {(data) => (
          <DataGrid<APIAuditLog>
            rows={data.items ?? []}
            columns={columns}
            getRowId={(row) => row.id}
            paginationMode="server"
            rowCount={rowCount}
            paginationModel={paginationModel}
            onPaginationModelChange={setPaginationModel}
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
              },
              "& .MuiDataGrid-footerContainer": {
                borderTop: "1px solid",
                borderColor: "divider",
              },
            }}
          />
        )}
      </QueryState>

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
