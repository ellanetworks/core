"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Box,
  Typography,
  Alert,
  Collapse,
  IconButton,
  Tooltip,
  Tabs,
  Tab,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  DataGrid,
  GridFilterModel,
  GridLogicOperator,
  getGridStringOperators,
  getGridDateOperators,
  getGridSingleSelectOperators,
  type GridColDef,
  type GridRenderCellParams,
  type GridPaginationModel,
  type GridFilterInputDateProps,
  type GridFilterOperator,
  type GridValidRowModel,
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
  listSubscriberLogs,
  clearSubscriberLogs,
  getSubscriberLogRetentionPolicy,
  type SubscriberLogRetentionPolicy,
  type APISubscriberLog,
  type ListSubscriberLogsResponse,
} from "@/queries/subscriber_logs";
import { useQuery, keepPreviousData } from "@tanstack/react-query";
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

const STRING_EQ = getGridStringOperators().filter(
  (op) => op.value === "equals",
);

const DIR_EQ = getGridSingleSelectOperators().filter((op) => op.value === "is");

type DateOpFor<Row extends GridValidRowModel> = GridFilterOperator<
  Row,
  Date,
  Date,
  GridFilterInputDateProps
>;

function makeTimestampOps<
  Row extends GridValidRowModel,
>(): readonly DateOpFor<Row>[] {
  const base = getGridDateOperators(true) as readonly GridFilterOperator<
    GridValidRowModel,
    Date,
    Date,
    GridFilterInputDateProps
  >[];

  const after = base.find((op) => op.value === "after");
  const before = base.find((op) => op.value === "before");

  const ops: DateOpFor<Row>[] = [];
  if (after)
    ops.push({ ...after, label: "After" } as unknown as DateOpFor<Row>);
  if (before)
    ops.push({ ...before, label: "Before" } as unknown as DateOpFor<Row>);
  return ops;
}

const TIMESTAMP_OPS_SUB = makeTimestampOps<APISubscriberLog>();
const TIMESTAMP_OPS_RADIO = makeTimestampOps<APIRadioLog>();

function toISOFromFilterValue(v: unknown): string | undefined {
  if (v instanceof Date) return v.toISOString();
  if (typeof v === "number" || typeof v === "string") {
    const d = new Date(v);
    return isNaN(d.getTime()) ? undefined : d.toISOString();
  }
  return undefined;
}

function filtersToParams(
  model: GridFilterModel,
): Record<string, string | string[]> {
  const items = model?.items ?? [];
  const bucket: Record<string, string[]> = {};
  let fromISO: string | undefined;
  let toISO: string | undefined;

  const ms = (iso: string) => new Date(iso).getTime();

  for (const { field, operator, value } of items) {
    if (!field || value == null || value === "") continue;

    if (field === "timestamp") {
      const iso = toISOFromFilterValue(value);
      if (!iso) continue;

      if (operator === "after") {
        if (!fromISO || ms(iso) > ms(fromISO)) fromISO = iso;
      } else if (operator === "before") {
        if (!toISO || ms(iso) < ms(toISO)) toISO = iso;
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
  if (fromISO) params.from = fromISO;
  if (toISO) params.to = toISO;
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

  const [tab, setTab] = useState<TabKey>("subscribers");

  // ---------------- Shared UI ----------------
  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });
  const [viewLogModalOpen, setViewLogModalOpen] = useState(false);
  const [selectedRow, setSelectedRow] = useState<LogRow | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(true);
  const visible = usePageVisible();

  // ---------------- Subscribers tab state ----------------
  const [subRows, setSubRows] = useState<APISubscriberLog[]>([]);
  const [subRowCount, setSubRowCount] = useState<number>(0);
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
  const subToolbarValue = React.useMemo<EventToolbarState>(
    () => ({
      canEdit,
      retentionDays: subRetentionPolicy?.days ?? "…",
      onEditRetention: () => setSubscriberEditModalOpen(true),
      onClearAll: () => setSubscriberClearModalOpen(true),
      isLive: autoRefresh,
      onToggleLive: () => setAutoRefresh((v) => !v),
    }),
    [canEdit, subRetentionPolicy?.days, autoRefresh],
  );
  const [subFilterModel, setSubFilterModel] = useState<GridFilterModel>({
    items: [],
  });

  const onSubFilterModelChange = useCallback(
    (m: GridFilterModel) => setSubFilterModel(m),
    [],
  );

  // ---------------- Radio tab state ----------------
  const [radioRows, setRadioRows] = useState<APIRadioLog[]>([]);
  const [radioRowCount, setRadioRowCount] = useState<number>(0);
  const [radioPagination, setRadioPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });
  const [isRadioClearModalOpen, setRadioClearModalOpen] = useState(false);
  const [radioRetentionPolicy, setRadioRetentionPolicy] =
    useState<RadioLogRetentionPolicy | null>(null);
  const radioToolbarValue = React.useMemo<EventToolbarState>(
    () => ({
      canEdit,
      retentionDays: radioRetentionPolicy?.days ?? "…",
      onEditRetention: () => setRadioEditModalOpen(true),
      onClearAll: () => setRadioClearModalOpen(true),
      isLive: autoRefresh,
      onToggleLive: () => setAutoRefresh((v) => !v),
    }),
    [canEdit, radioRetentionPolicy?.days, autoRefresh],
  );
  const onRadioFilterModelChange = useCallback(
    (m: GridFilterModel) => setRadioFilterModel(m),
    [],
  );
  const [radioFilterModel, setRadioFilterModel] = useState<GridFilterModel>({
    items: [],
  });

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

  const subQuery = useQuery<ListSubscriberLogsResponse>({
    queryKey: [
      "subscriberLogs",
      subPagination.page,
      subPagination.pageSize,
      filtersToParams(subFilterModel),
      accessToken,
    ],
    enabled: tab === "subscribers" && authReady && !!accessToken,
    refetchInterval: autoRefresh && visible ? 3000 : false,
    placeholderData: keepPreviousData,
    queryFn: async () => {
      const pageOne = subPagination.page + 1;
      const filterParams = filtersToParams(subFilterModel);
      return listSubscriberLogs(
        accessToken!,
        pageOne,
        subPagination.pageSize,
        filterParams,
      );
    },
  });

  useEffect(() => {
    if (tab !== "subscribers" || !subQuery.data) return;
    setSubRows(subQuery.data.items ?? []);
    setSubRowCount(subQuery.data.total_count ?? 0);
  }, [tab, subQuery.data]);

  const radioQuery = useQuery<ListRadioLogsResponse>({
    queryKey: [
      "radioLogs",
      radioPagination.page,
      radioPagination.pageSize,
      filtersToParams(radioFilterModel),
      accessToken,
    ],
    enabled: tab === "radio" && authReady && !!accessToken,
    refetchInterval: autoRefresh && visible ? 3000 : false,
    placeholderData: keepPreviousData,
    queryFn: async () => {
      const pageOne = radioPagination.page + 1;
      const filterParams = filtersToParams(radioFilterModel);
      return listRadioLogs(
        accessToken!,
        pageOne,
        radioPagination.pageSize,
        filterParams,
      );
    },
  });

  useEffect(() => {
    if (tab !== "radio" || !radioQuery.data) return;
    setRadioRows(radioQuery.data.items ?? []);
    setRadioRowCount(radioQuery.data.total_count ?? 0);
  }, [tab, radioQuery.data]);

  const handleConfirmDeleteSubscriberLogs = async () => {
    setSubscriberClearModalOpen(false);
    if (!accessToken) return;
    try {
      await clearSubscriberLogs(accessToken);
      setAlert({
        message: `All subscriber logs cleared successfully!`,
        severity: "success",
      });
      subQuery.refetch();
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
      radioQuery.refetch();
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

  // ---------------- Columns ----------------
  const subscriberColumns: GridColDef<APISubscriberLog>[] = useMemo(
    () => [
      {
        field: "timestamp",
        headerName: "Timestamp",
        type: "dateTime",
        flex: 1,
        minWidth: 220,
        valueGetter: ({ value }) => (value ? new Date(String(value)) : null),
        sortable: false,
        filterOperators: TIMESTAMP_OPS_SUB,
      },
      {
        field: "imsi",
        headerName: "IMSI",
        flex: 1,
        minWidth: 220,
        sortable: false,
        filterOperators: STRING_EQ,
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
        field: "event",
        headerName: "Event",
        flex: 1,
        minWidth: 200,
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
    ],
    [],
  );

  const radioColumns: GridColDef<APIRadioLog>[] = useMemo(
    () => [
      {
        field: "timestamp",
        headerName: "Timestamp",
        type: "dateTime",
        flex: 1,
        minWidth: 220,
        valueGetter: ({ value }) => (value ? new Date(String(value)) : null),
        sortable: false,
        filterOperators: TIMESTAMP_OPS_RADIO,
      },
      {
        field: "ran_id",
        headerName: "RAN ID",
        flex: 1,
        minWidth: 180,
        sortable: false,
        filterOperators: STRING_EQ,
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
        field: "event",
        headerName: "Event",
        flex: 1,
        minWidth: 200,
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
          </Box>

          <Box
            sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}
          >
            <ThemeProvider theme={gridTheme}>
              <EventToolbarContext.Provider value={subToolbarValue}>
                <DataGrid<APISubscriberLog>
                  rows={subRows}
                  columns={subscriberColumns}
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
          </Box>

          <Box
            sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}
          >
            <ThemeProvider theme={gridTheme}>
              <EventToolbarContext.Provider value={radioToolbarValue}>
                <DataGrid<APIRadioLog>
                  rows={radioRows}
                  columns={radioColumns}
                  getRowId={(row) => row.id}
                  loading={radioQuery.isFetching}
                  paginationMode="server"
                  rowCount={radioRowCount}
                  paginationModel={radioPagination}
                  onPaginationModelChange={setRadioPagination}
                  disableRowSelectionOnClick
                  disableColumnMenu
                  sortingMode="server"
                  filterMode="server"
                  onFilterModelChange={onRadioFilterModelChange}
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
              </EventToolbarContext.Provider>
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
        initialData={subRetentionPolicy || { days: 7 }}
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
        initialData={radioRetentionPolicy || { days: 7 }}
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
