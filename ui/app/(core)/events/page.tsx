"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Box,
  Typography,
  Alert,
  Chip,
  Collapse,
  IconButton,
  Tooltip,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  DataGrid,
  GridFilterModel,
  GridLogicOperator,
  getGridStringOperators,
  getGridSingleSelectOperators,
  getGridDateOperators,
  type GridFilterOperator,
  type GridColDef,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import VisibilityIcon from "@mui/icons-material/Visibility";
import EastIcon from "@mui/icons-material/East";
import WestIcon from "@mui/icons-material/West";
import {
  EventToolbar,
  EventToolbarContext,
  type EventToolbarState,
} from "@/components/EventToolbar";
import {
  listNetworkLogs,
  clearNetworkLogs,
  getNetworkLogRetentionPolicy,
  type NetworkLogRetentionPolicy,
  type APINetworkLog,
  type ListNetworkLogsResponse,
} from "@/queries/network_logs";
import { useQuery, keepPreviousData } from "@tanstack/react-query";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { useAuth } from "@/contexts/AuthContext";
import EditNetworkLogRetentionPolicyModal from "@/components/EditNetworkLogRetentionPolicyModal";
import ViewLogModal from "@/components/ViewLogModal";
import type { LogRow } from "@/components/ViewLogModal";

const MAX_WIDTH = 1400;

const STRING_EQ = getGridStringOperators().filter(
  (op) => op.value === "equals",
);

const DIR_EQ = getGridSingleSelectOperators().filter((op) => op.value === "is");

const PROTOCOL_EQ = getGridSingleSelectOperators().filter(
  (op) => op.value === "is",
);

// turns "2025-10-09T09:34:27.496-0400" into "...-04:00"
const normalizeRfc3339Offset = (s: string) =>
  s.replace(/([+-]\d{2})(\d{2})$/, "$1:$2");

type GridNetworkLog = APINetworkLog & { timestamp_dt: Date | null };

const DATE_AFTER_BEFORE_ONLY = getGridDateOperators(true).filter(
  (op) => op.value === "after" || op.value === "before",
) as unknown as readonly GridFilterOperator[];

function formatRfc3339WithOffset(d: Date): string {
  const pad = (n: number, len = 2) => String(Math.abs(n)).padStart(len, "0");
  const y = d.getFullYear();
  const m = pad(d.getMonth() + 1);
  const day = pad(d.getDate());
  const hh = pad(d.getHours());
  const mm = pad(d.getMinutes());
  const ss = pad(d.getSeconds());
  const ms = pad(d.getMilliseconds(), 3);

  const tzMin = -d.getTimezoneOffset();
  const sign = tzMin >= 0 ? "+" : "-";
  const tzH = pad(Math.trunc(Math.abs(tzMin) / 60));
  const tzM = pad(Math.abs(tzMin) % 60);

  return `${y}-${m}-${day}T${hh}:${mm}:${ss}.${ms}${sign}${tzH}:${tzM}`;
}

const ProtocolCell: React.FC<{ value?: string }> = ({ value }) => {
  if (!value) return null;

  const val = String(value).toUpperCase();
  const color = val === "NGAP" ? "info" : val === "NAS" ? "success" : "default";

  return (
    <Chip
      label={val}
      size="small"
      variant={color === "default" ? "outlined" : "filled"}
      color={color}
      sx={{
        fontWeight: 600,
        letterSpacing: 0.25,
        height: 22,
      }}
      aria-label={`Protocol ${val}`}
    />
  );
};

function toBackendTimestamp(v: unknown): string | undefined {
  if (v instanceof Date) return formatRfc3339WithOffset(v);
  if (typeof v === "string" || typeof v === "number") {
    const d = new Date(v);
    return Number.isNaN(d.getTime()) ? undefined : formatRfc3339WithOffset(d);
  }
  return undefined;
}

function filtersToParams(
  model: GridFilterModel,
): Record<string, string | string[]> {
  const items = model?.items ?? [];
  const bucket: Record<string, string[]> = {};
  let timestampFromISO: string | undefined;
  let timestampToISO: string | undefined;

  const ms = (iso: string) => new Date(iso).getTime();

  for (const { field, operator, value } of items) {
    if (!field || value == null || value === "") continue;

    if (field === "timestamp" || field === "timestamp_dt") {
      const iso = toBackendTimestamp(value);
      if (!iso) continue;
      if (operator === "after") {
        if (!timestampFromISO || ms(iso) > ms(timestampFromISO))
          timestampFromISO = iso;
      } else if (operator === "before") {
        if (!timestampToISO || ms(iso) < ms(timestampToISO))
          timestampToISO = iso;
      }
      continue;
    }

    const arr = Array.isArray(value) ? value.map(String) : [String(value)];
    bucket[field] = (bucket[field] ?? []).concat(arr);
  }

  const params: Record<string, string | string[]> = {};
  for (const k of Object.keys(bucket)) {
    params[k] = bucket[k].length === 1 ? bucket[k][0] : bucket[k];
  }
  if (timestampFromISO) params.timestamp_from = timestampFromISO;
  if (timestampToISO) params.timestamp_to = timestampToISO;
  return params;
}

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

function usePageVisible() {
  const [visible, setVisible] = React.useState(!document.hidden);
  useEffect(() => {
    const onVis = () => setVisible(!document.hidden);
    document.addEventListener("visibilitychange", onVis);
    return () => document.removeEventListener("visibilitychange", onVis);
  }, []);
  return visible;
}

const Events: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin";

  const outerTheme = useTheme();
  const gridTheme = useMemo(() => createTheme(outerTheme), [outerTheme]);

  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });
  const [viewLogModalOpen, setViewLogModalOpen] = useState(false);
  const [selectedRow, setSelectedRow] = useState<LogRow | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const visible = usePageVisible();

  const [networkRows, setSubRows] = useState<APINetworkLog[]>([]);
  const [subRowCount, setSubRowCount] = useState<number>(0);
  const [subPagination, setSubPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [isNetworkEditModalOpen, setNetworkEditModalOpen] = useState(false);
  const [isNetworkClearModalOpen, setNetworkClearModalOpen] = useState(false);
  const [networkRetentionPolicy, setSubRetentionPolicy] =
    useState<NetworkLogRetentionPolicy | null>(null);
  const subToolbarValue = React.useMemo<EventToolbarState>(
    () => ({
      canEdit,
      retentionDays: networkRetentionPolicy?.days ?? "…",
      onEditRetention: () => setNetworkEditModalOpen(true),
      onClearAll: () => setNetworkClearModalOpen(true),
      isLive: autoRefresh,
      onToggleLive: () => setAutoRefresh((v) => !v),
    }),
    [canEdit, networkRetentionPolicy?.days, autoRefresh],
  );
  const [networkFilterModel, setSubFilterModel] = useState<GridFilterModel>({
    items: [],
  });

  const onSubFilterModelChange = useCallback(
    (m: GridFilterModel) => setSubFilterModel(m),
    [],
  );

  const fetchNetworkRetention = useCallback(async () => {
    if (!authReady || !accessToken) return;
    try {
      const data = await getNetworkLogRetentionPolicy(accessToken);
      setSubRetentionPolicy(data);
    } catch (e) {
      console.error("Error fetching network log retention policy:", e);
    }
  }, [accessToken, authReady]);

  const subQuery = useQuery<ListNetworkLogsResponse>({
    queryKey: [
      "networkLogs",
      subPagination.page,
      subPagination.pageSize,
      filtersToParams(networkFilterModel),
      accessToken,
    ],
    enabled: authReady && !!accessToken,
    refetchInterval: autoRefresh && visible ? 3000 : false,
    placeholderData: keepPreviousData,
    queryFn: async () => {
      const pageOne = subPagination.page + 1;
      const filterParams = filtersToParams(networkFilterModel);
      return listNetworkLogs(
        accessToken!,
        pageOne,
        subPagination.pageSize,
        filterParams,
      );
    },
  });

  useEffect(() => {
    if (!subQuery.data) return;
    const items = (subQuery.data.items ?? []).map<GridNetworkLog>((r) => ({
      ...r,
      timestamp_dt: r.timestamp
        ? new Date(normalizeRfc3339Offset(r.timestamp))
        : null,
    }));
    setSubRows(items);
    setSubRowCount(subQuery.data.total_count ?? 0);
  }, [subQuery.data]);

  const handleConfirmDeleteNetworkLogs = async () => {
    setNetworkClearModalOpen(false);
    if (!accessToken) return;
    try {
      await clearNetworkLogs(accessToken);
      setAlert({
        message: `All network logs cleared successfully!`,
        severity: "success",
      });
      subQuery.refetch();
    } catch (error: unknown) {
      setAlert({
        message: `Failed to clear network logs: ${String(error)}`,
        severity: "error",
      });
    }
  };

  useEffect(() => {
    fetchNetworkRetention();
  }, [fetchNetworkRetention]);

  const networkColumns: GridColDef<APINetworkLog>[] = useMemo(() => {
    return [
      {
        field: "timestamp_dt",
        headerName: "Timestamp",
        type: "dateTime",
        flex: 1,
        minWidth: 220,
        sortable: false,
        renderCell: (p) => (p.value ? p.value.toLocaleString() : ""),
        filterOperators: DATE_AFTER_BEFORE_ONLY,
      },
      {
        field: "direction",
        headerName: "Dir",
        width: 70,
        align: "center",
        headerAlign: "center",
        sortable: false,
        type: "singleSelect",
        valueOptions: [
          { value: "inbound", label: "Inbound" },
          { value: "outbound", label: "Outbound" },
        ],
        filterOperators: DIR_EQ,
        renderCell: (p) => <DirectionCell value={p.row.direction} />,
      },
      {
        field: "protocol",
        headerName: "Protocol",
        type: "singleSelect",
        valueOptions: [
          { value: "NGAP", label: "NGAP" },
          { value: "NAS", label: "NAS" },
        ],
        flex: 1,
        minWidth: 120,
        sortable: false,
        filterOperators: PROTOCOL_EQ,
        renderCell: (p) => <ProtocolCell value={p.row.protocol} />,
      },
      {
        field: "message_type",
        headerName: "Message Type",
        flex: 1,
        minWidth: 220,
        sortable: false,
        filterOperators: STRING_EQ,
      },
      {
        field: "view",
        headerName: "",
        sortable: false,
        filterable: false,
        width: 60,
        align: "center",
        headerAlign: "center",
        renderCell: (params) => (
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
                  protocol: r.protocol,
                  messageType: r.message_type,
                  direction: r.direction,
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
    ];
  }, []);

  const subDescription =
    "Review network events in Ella Core. These logs are useful for auditing and troubleshooting purposes.";

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
          <Typography variant="h4">Network Events</Typography>
          <Typography variant="body1" color="text.secondary">
            {subDescription}
          </Typography>
        </Box>

        <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}>
          <ThemeProvider theme={gridTheme}>
            <EventToolbarContext.Provider value={subToolbarValue}>
              <DataGrid<APINetworkLog>
                rows={networkRows}
                columns={networkColumns}
                getRowId={(row) => row.id}
                loading={subQuery.isFetching}
                paginationMode="server"
                rowCount={subRowCount}
                paginationModel={subPagination}
                onPaginationModelChange={setSubPagination}
                disableRowSelectionOnClick
                disableColumnMenu
                sortingMode="server"
                filterMode="server"
                onFilterModelChange={onSubFilterModelChange}
                pageSizeOptions={[10, 25, 50, 100]}
                slots={{ toolbar: EventToolbar }}
                slotProps={{
                  filterPanel: {
                    disableAddFilterButton: false,
                    disableRemoveAllButton: false,
                    logicOperators: [GridLogicOperator.And],
                    filterFormProps: {
                      logicOperatorInputProps: { sx: { display: "none" } },
                    },
                  },
                }}
                showToolbar
                sx={{
                  border: 1,
                  borderColor: "divider",
                  "& .MuiDataGrid-columnHeaders": { borderTop: 0 },
                  "& .MuiDataGrid-footerContainer": {
                    borderTop: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-columnHeaderTitle": { fontWeight: "bold" },
                }}
              />
            </EventToolbarContext.Provider>
          </ThemeProvider>
        </Box>
      </>

      <ViewLogModal
        open={viewLogModalOpen}
        onClose={() => setViewLogModalOpen(false)}
        log={selectedRow}
      />
      <EditNetworkLogRetentionPolicyModal
        open={isNetworkEditModalOpen}
        onClose={() => setNetworkEditModalOpen(false)}
        onSuccess={() => {
          fetchNetworkRetention();
          setAlert({
            message: "Retention policy updated!",
            severity: "success",
          });
        }}
        initialData={networkRetentionPolicy || { days: 7 }}
      />
      <DeleteConfirmationModal
        title="Clear All Network Logs"
        description="Are you sure you want to clear all network logs? This action cannot be undone."
        open={isNetworkClearModalOpen}
        onClose={() => setNetworkClearModalOpen(false)}
        onConfirm={handleConfirmDeleteNetworkLogs}
      />
    </Box>
  );
};

export default Events;
