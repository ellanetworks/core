"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Box,
  Typography,
  Alert,
  Chip,
  Collapse,
  Tooltip,
  IconButton,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  DataGrid,
  GridFilterModel,
  getGridStringOperators,
  getGridSingleSelectOperators,
  getGridDateOperators,
  type GridFilterOperator,
  type GridColDef,
  type GridPaginationModel,
  type GridRowParams,
  type GridRowId,
  type GridRowSelectionModel,
} from "@mui/x-data-grid";
import EastIcon from "@mui/icons-material/East";
import WestIcon from "@mui/icons-material/West";
import CloseIcon from "@mui/icons-material/Close";
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
import EventDetails from "@/components/EventDetails";
import type { LogRow } from "@/components/EventDetails";
import { Panel, PanelGroup, PanelResizeHandle } from "react-resizable-panels";
import DragIndicatorIcon from "@mui/icons-material/DragIndicator";

const MAX_WIDTH = 1400;
const PAGE_PAD = { xs: 2, sm: 4 };

const STRING_EQ = getGridStringOperators().filter(
  (op) => op.value === "equals",
);
const DIR_EQ = getGridSingleSelectOperators().filter((op) => op.value === "is");
const PROTOCOL_EQ = getGridSingleSelectOperators().filter(
  (op) => op.value === "is",
);
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
  const styles =
    val === "NGAP"
      ? { backgroundColor: "#003366", color: "#fff" }
      : val === "NAS"
        ? { backgroundColor: "#ff7300ff", color: "#fff" }
        : {
            backgroundColor: "transparent",
            color: "text.primary",
            border: "1px solid",
            borderColor: "divider",
          };
  return (
    <Chip
      label={val}
      size="small"
      sx={{ fontWeight: 600, letterSpacing: 0.25, height: 22, ...styles }}
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

const ResizeHandle: React.FC = React.memo(function ResizeHandle() {
  return (
    <PanelResizeHandle>
      <Box
        sx={{
          width: 16,
          height: "100%",
          cursor: "ew-resize",
          position: "relative",
          zIndex: (t) => t.zIndex.appBar,
          "&:hover .resizeIcon": { opacity: 1 },
        }}
        tabIndex={0}
        role="separator"
        aria-orientation="vertical"
        aria-label="Resize details panel"
      >
        <Box
          sx={{
            position: "sticky",
            top: "calc(50vh - 12px)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            pointerEvents: "none",
          }}
        >
          <DragIndicatorIcon
            className="resizeIcon"
            sx={{
              fontSize: 24,
              opacity: 0.7,
              transition: "opacity 120ms",
              color: "text.secondary",
            }}
          />
        </Box>
      </Box>
    </PanelResizeHandle>
  );
});

const Events: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin";

  const outerTheme = useTheme();
  const gridTheme = useMemo(() => createTheme(outerTheme), [outerTheme]);

  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });
  const [viewEventDrawerOpen, setViewEventDrawerOpen] = useState(false);
  const [selectedRow, setSelectedRow] = useState<LogRow | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const visible = usePageVisible();
  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const makeSelection = (ids: GridRowId[] = []): GridRowSelectionModel => ({
    type: "include",
    ids: new Set<GridRowId>(ids),
  });
  const [selectionModel, setSelectionModel] =
    useState<GridRowSelectionModel>(makeSelection());

  const [isNetworkEditModalOpen, setNetworkEditModalOpen] = useState(false);
  const [isNetworkClearModalOpen, setNetworkClearModalOpen] = useState(false);

  const retentionQuery = useQuery<NetworkLogRetentionPolicy>({
    queryKey: ["networkLogRetention", accessToken],
    enabled: authReady && !!accessToken && !isNetworkEditModalOpen,
    queryFn: () => getNetworkLogRetentionPolicy(accessToken!),
  });

  const subToolbarValue = React.useMemo<EventToolbarState>(
    () => ({
      canEdit,
      retentionDays: retentionQuery.data?.days ?? "…",
      onEditRetention: () => setNetworkEditModalOpen(true),
      onClearAll: () => setNetworkClearModalOpen(true),
      isLive: autoRefresh,
      onToggleLive: () => setAutoRefresh((v) => !v),
    }),
    [canEdit, retentionQuery.data?.days, autoRefresh],
  );
  const [networkFilterModel, setSubFilterModel] = useState<GridFilterModel>({
    items: [],
  });
  const onSubFilterModelChange = useCallback(
    (m: GridFilterModel) => setSubFilterModel(m),
    [],
  );

  const pageOneBased = paginationModel.page + 1;
  const perPage = paginationModel.pageSize;

  const networkLogsQuery = useQuery<ListNetworkLogsResponse>({
    queryKey: [
      "networkLogs",
      pageOneBased,
      perPage,
      filtersToParams(networkFilterModel),
      accessToken,
    ],
    enabled: authReady && !!accessToken,
    refetchInterval: autoRefresh && visible ? 3000 : false,
    placeholderData: keepPreviousData,
    queryFn: async () => {
      const filterParams = filtersToParams(networkFilterModel);
      return listNetworkLogs(accessToken!, pageOneBased, perPage, filterParams);
    },
  });

  const networkRows: GridNetworkLog[] = useMemo(() => {
    const items = networkLogsQuery.data?.items ?? [];
    return items.map<GridNetworkLog>((r) => ({
      ...r,
      timestamp_dt: r.timestamp
        ? new Date(normalizeRfc3339Offset(r.timestamp))
        : null,
    }));
  }, [networkLogsQuery.data?.items]);

  const subRowCount = networkLogsQuery.data?.total_count ?? 0;

  const handleConfirmDeleteNetworkLogs = async () => {
    setNetworkClearModalOpen(false);
    if (!accessToken) return;
    try {
      await clearNetworkLogs(accessToken);
      setAlert({
        message: `All network logs cleared successfully!`,
        severity: "success",
      });
      networkLogsQuery.refetch();
    } catch (error: unknown) {
      setAlert({
        message: `Failed to clear network logs: ${String(error)}`,
        severity: "error",
      });
    }
  };

  const networkColumns: GridColDef<APINetworkLog>[] = useMemo(() => {
    return [
      {
        field: "timestamp_dt",
        headerName: "Timestamp",
        type: "dateTime",
        flex: 1,
        minWidth: 180,
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
        width: 120,
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
        field: "local_address",
        headerName: "Local Address",
        flex: 1,
        minWidth: 150,
        sortable: false,
        filterOperators: STRING_EQ,
      },
      {
        field: "remote_address",
        headerName: "Remote Address",
        flex: 1,
        minWidth: 150,
        sortable: false,
        filterOperators: STRING_EQ,
      },
    ];
  }, []);

  const handleRowClick = useCallback((params: GridRowParams<APINetworkLog>) => {
    const r = params.row;
    setSelectionModel(makeSelection([params.id]));
    setSelectedRow({
      id: String(r.id),
      timestamp: r.timestamp,
      protocol: r.protocol,
      messageType: r.message_type,
      direction: r.direction,
      local_address: r.local_address,
      remote_address: r.remote_address,
    });
    setViewEventDrawerOpen(true);
  }, []);

  const subDescription =
    "Review network events in Ella Core. These logs are useful for auditing and troubleshooting purposes.";

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setViewEventDrawerOpen(false);
    };
    if (viewEventDrawerOpen) window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [viewEventDrawerOpen]);

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
      <PanelGroup
        direction="horizontal"
        style={{ width: "100%", height: "100%", overflow: "hidden" }}
      >
        <Panel minSize={20}>
          <Box sx={{ maxWidth: MAX_WIDTH, mx: "auto" }}>
            <Collapse in={!!alert.message}>
              <Alert
                severity={alert.severity || "success"}
                onClose={() => setAlert({ message: "", severity: null })}
                sx={{ mb: 2, pointerEvents: "auto" }}
              >
                {alert.message}
              </Alert>
            </Collapse>
          </Box>
          <Box
            sx={{
              height: "100%",
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              overflow: "hidden",
            }}
          >
            <Box
              sx={{
                width: "100%",
                maxWidth: viewEventDrawerOpen ? "none" : MAX_WIDTH,
                pb: 4,
                display: "flex",
                flexDirection: "column",
                gap: 2,
                minWidth: 0,
                height: "100%",
                overflow: "hidden",
                pl: PAGE_PAD,
                pr: viewEventDrawerOpen ? 0 : PAGE_PAD,
              }}
            >
              <Box sx={{ flexShrink: 0 }}>
                <Typography variant="h4">Network Events</Typography>
                <Typography variant="body1" color="text.secondary">
                  {subDescription}
                </Typography>
              </Box>

              <Box sx={{ flex: 1, minHeight: 0 }}>
                <ThemeProvider theme={gridTheme}>
                  <EventToolbarContext.Provider value={subToolbarValue}>
                    <DataGrid<APINetworkLog>
                      rows={networkRows}
                      columns={networkColumns}
                      getRowId={(row) => row.id}
                      paginationMode="server"
                      rowCount={subRowCount}
                      paginationModel={paginationModel}
                      onPaginationModelChange={setPaginationModel}
                      disableColumnMenu
                      sortingMode="server"
                      filterMode="server"
                      onFilterModelChange={onSubFilterModelChange}
                      pageSizeOptions={[10, 25, 50, 100]}
                      slots={{ toolbar: EventToolbar }}
                      onRowClick={handleRowClick}
                      rowSelectionModel={selectionModel}
                      disableRowSelectionOnClick
                      onRowSelectionModelChange={(model) =>
                        setSelectionModel(model)
                      }
                      showToolbar
                      density="compact"
                      autoHeight
                      sx={{
                        border: 1,
                        borderColor: "divider",
                        height: "100%",
                        "& .MuiDataGrid-columnHeaders": { borderTop: 0 },
                        "& .MuiDataGrid-footerContainer": {
                          borderTop: "1px solid",
                          borderColor: "divider",
                        },
                        "& .MuiDataGrid-columnHeaderTitle": {
                          fontWeight: "bold",
                        },
                        "& .MuiDataGrid-row:hover": { cursor: "pointer" },
                        "& .MuiDataGrid-row.Mui-selected": {
                          backgroundColor: (t) => t.palette.action.selected,
                          "&:hover": {
                            backgroundColor: (t) => t.palette.action.selected,
                          },
                          "& .MuiDataGrid-cell": { fontWeight: 500 },
                          "&::before": { display: "none" },
                        },
                        "& .MuiDataGrid-cell:focus, & .MuiDataGrid-cell:focus-within":
                          { outline: "none" },
                        "& .MuiDataGrid-columnHeader:focus, & .MuiDataGrid-columnHeader:focus-within":
                          { outline: "none" },
                      }}
                    />
                  </EventToolbarContext.Provider>
                </ThemeProvider>
              </Box>
            </Box>
          </Box>
        </Panel>

        {viewEventDrawerOpen && <ResizeHandle />}

        {viewEventDrawerOpen && (
          <Panel defaultSize={45} minSize={30} maxSize={70}>
            <Box
              sx={{
                height: "100%",
                display: "flex",
                flexDirection: "column",
                bgcolor: "background.paper",
                borderLeft: (t) => `1px solid ${t.palette.divider}`,
                pl: PAGE_PAD,
              }}
            >
              <Box
                sx={{
                  px: 0,
                  py: 1.5,
                  borderBottom: (t) => `1px solid ${t.palette.divider}`,
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                }}
              >
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Typography variant="h6" noWrap>
                    {selectedRow?.messageType ?? "Event details"}
                  </Typography>
                </Box>
                <IconButton
                  aria-label="Close"
                  onClick={() => setViewEventDrawerOpen(false)}
                  size="small"
                >
                  <CloseIcon />
                </IconButton>
              </Box>

              {/* Your existing details content */}
              <EventDetails open={viewEventDrawerOpen} log={selectedRow} />
            </Box>
          </Panel>
        )}
      </PanelGroup>

      {/* Modals */}
      <EditNetworkLogRetentionPolicyModal
        open={isNetworkEditModalOpen}
        onClose={() => setNetworkEditModalOpen(false)}
        onSuccess={() => {
          retentionQuery.refetch();
          setAlert({
            message: "Retention policy updated!",
            severity: "success",
          });
        }}
        initialDays={retentionQuery.data?.days || 7}
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
