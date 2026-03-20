import React, { useMemo, useState, useEffect, useCallback } from "react";
import {
  Box,
  Button,
  Typography,
  Chip,
  CircularProgress,
  Tabs,
  Tab,
  Tooltip,
  IconButton,
  TextField,
  MenuItem,
} from "@mui/material";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import {
  DataGrid,
  type GridColDef,
  type GridPaginationModel,
  type GridRowParams,
  type GridRowId,
  type GridRowSelectionModel,
} from "@mui/x-data-grid";
import { useSearchParams, Link } from "react-router-dom";
import EastIcon from "@mui/icons-material/East";
import WestIcon from "@mui/icons-material/West";
import CloseIcon from "@mui/icons-material/Close";
import DragIndicatorIcon from "@mui/icons-material/DragIndicator";
import DeleteOutlineIcon from "@mui/icons-material/DeleteOutline";
import { Edit as EditIcon } from "@mui/icons-material";
import { Panel, PanelGroup, PanelResizeHandle } from "react-resizable-panels";

import {
  listRadios,
  type APIRadio,
  type ListRadiosResponse,
} from "@/queries/radios";
import EmptyState from "@/components/EmptyState";
import { useAuth } from "@/contexts/AuthContext";
import { useQuery, keepPreviousData } from "@tanstack/react-query";

import {
  listRadioEvents,
  clearRadioEvents,
  getRadioEventRetentionPolicy,
  type RadioEventRetentionPolicy,
  type APIRadioEvent,
  type ListRadioEventsResponse,
} from "@/queries/radio_events";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EditRadioEventRetentionPolicyModal from "@/components/EditRadioEventRetentionPolicyModal";
import EventDetails from "@/components/EventDetails";
import type { LogRow } from "@/components/EventDetails";
import { MAX_WIDTH, PAGE_PADDING_X as PAGE_PAD } from "@/utils/layout";

type TabKey = "radios" | "events";

// -------- Helpers & small components from old Events page --------

const NGAP_MESSAGE_TYPES = [
  "AMFConfigurationUpdate",
  "AMFConfigurationUpdateAcknowledge",
  "AMFConfigurationUpdateFailure",
  "AMFStatusIndication",
  "CellTrafficTrace",
  "DeactivateTrace",
  "DownlinkNASTransport",
  "DownlinkNonUEAssociatedNRPPaTransport",
  "DownlinkRANConfigurationTransfer",
  "DownlinkRANStatusTransfer",
  "DownlinkUEAssociatedNRPPaTransport",
  "ErrorIndication",
  "HandoverCancel",
  "HandoverCancelAcknowledge",
  "HandoverCommand",
  "HandoverFailure",
  "HandoverNotify",
  "HandoverPreparationFailure",
  "HandoverRequest",
  "HandoverRequestAcknowledge",
  "HandoverRequired",
  "InitialContextSetupFailure",
  "InitialContextSetupRequest",
  "InitialContextSetupResponse",
  "InitialUEMessage",
  "LocationReport",
  "LocationReportingControl",
  "LocationReportingFailureIndication",
  "NASNonDeliveryIndication",
  "NGReset",
  "NGResetAcknowledge",
  "NGSetupFailure",
  "NGSetupRequest",
  "NGSetupResponse",
  "OverloadStart",
  "OverloadStop",
  "Paging",
  "PathSwitchRequest",
  "PathSwitchRequestAcknowledge",
  "PathSwitchRequestFailure",
  "PDUSessionResourceModifyConfirm",
  "PDUSessionResourceModifyIndication",
  "PDUSessionResourceModifyRequest",
  "PDUSessionResourceModifyResponse",
  "PDUSessionResourceNotify",
  "PDUSessionResourceReleaseCommand",
  "PDUSessionResourceReleaseResponse",
  "PDUSessionResourceSetupRequest",
  "PDUSessionResourceSetupResponse",
  "PrivateMessage",
  "PWSCancelRequest",
  "PWSCancelResponse",
  "PWSFailureIndication",
  "PWSRestartIndication",
  "RANConfigurationUpdate",
  "RANConfigurationUpdateAcknowledge",
  "RANConfigurationUpdateFailure",
  "RerouteNASRequest",
  "RRCInactiveTransitionReport",
  "SecondaryRATDataUsageReport",
  "TraceFailureIndication",
  "TraceStart",
  "UEContextModificationFailure",
  "UEContextModificationRequest",
  "UEContextModificationResponse",
  "UEContextReleaseCommand",
  "UEContextReleaseComplete",
  "UEContextReleaseRequest",
  "UERadioCapabilityCheckRequest",
  "UERadioCapabilityCheckResponse",
  "UERadioCapabilityInfoIndication",
  "UETNLABindingReleaseRequest",
  "UplinkNASTransport",
  "UplinkNonUEAssociatedNRPPaTransport",
  "UplinkRANConfigurationTransfer",
  "UplinkRANStatusTransfer",
  "UplinkUEAssociatedNRPPaTransport",
  "WriteReplaceWarningRequest",
  "WriteReplaceWarningResponse",
];
const normalizeRfc3339Offset = (s: string) =>
  s.replace(/([+-]\d{2})(\d{2})$/, "$1:$2");

type GridRadioEvent = APIRadioEvent & { timestamp_dt: Date | null };

const DirectionCell: React.FC<{ value?: string }> = ({ value }) => {
  const theme = useTheme();
  if (!value) return null;
  const Icon = value === "outbound" ? EastIcon : WestIcon;
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
  const [visible, setVisible] = useState(
    typeof document === "undefined" ? true : !document.hidden,
  );
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

// ------------------- Events tab content (moved from old page) -------------------

const EventsTab: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin";
  const theme = useTheme();

  const { showSnackbar } = useSnackbar();
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

  // URL params for deep-linking
  const [searchParams] = useSearchParams();
  const radioParam = searchParams.get("radio") ?? "";
  const eventIdParam = searchParams.get("event");

  // Explicit filter state (replaces DataGrid built-in filter model)
  const [radioFilter, setRadioFilter] = useState(radioParam);
  const [directionFilter, setDirectionFilter] = useState("");
  const [messageTypeFilter, setMessageTypeFilter] = useState("");

  // Re-sync radio filter when URL param changes
  useEffect(() => {
    setRadioFilter(radioParam);
  }, [radioParam]);

  // Fetch radios list for filter dropdown
  const radiosQuery = useQuery<ListRadiosResponse>({
    queryKey: ["radios-for-filter"],
    queryFn: () => listRadios(accessToken!, 1, 100),
    enabled: authReady && !!accessToken,
    refetchInterval: 10_000,
  });
  const radioOptions: APIRadio[] = radiosQuery.data?.items ?? [];

  const retentionQuery = useQuery<RadioEventRetentionPolicy>({
    queryKey: ["networkLogRetention"],
    enabled: authReady && !!accessToken && !isNetworkEditModalOpen,
    queryFn: () => getRadioEventRetentionPolicy(accessToken!),
  });

  const pageOneBased = paginationModel.page + 1;
  const perPage = paginationModel.pageSize;

  // Build query params from explicit filters
  const filterParams = useMemo(() => {
    const params: Record<string, string> = {};
    if (radioFilter) params.radio = radioFilter;
    if (directionFilter) params.direction = directionFilter;
    if (messageTypeFilter) params.message_type = messageTypeFilter;
    return params;
  }, [radioFilter, directionFilter, messageTypeFilter]);

  const networkLogsQuery = useQuery<ListRadioEventsResponse>({
    queryKey: ["networkLogs", pageOneBased, perPage, filterParams],
    enabled: authReady && !!accessToken,
    refetchInterval: autoRefresh && visible ? 3000 : false,
    placeholderData: keepPreviousData,
    queryFn: () =>
      listRadioEvents(accessToken!, pageOneBased, perPage, filterParams),
  });

  const networkRows: GridRadioEvent[] = useMemo(() => {
    const items = networkLogsQuery.data?.items ?? [];
    return items.map<GridRadioEvent>((r) => ({
      ...r,
      timestamp_dt: r.timestamp
        ? new Date(normalizeRfc3339Offset(r.timestamp))
        : null,
    }));
  }, [networkLogsQuery.data?.items]);

  const subRowCount = networkLogsQuery.data?.total_count ?? 0;

  // Deep-link: when event param is present, find and select matching row
  useEffect(() => {
    if (!eventIdParam || !networkLogsQuery.data?.items) return;
    const eventId = Number(eventIdParam);
    const match = networkLogsQuery.data.items.find((r) => r.id === eventId);
    if (match) {
      setSelectionModel(makeSelection([match.id]));
      setSelectedRow({
        id: String(match.id),
        timestamp: match.timestamp,
        protocol: match.protocol,
        messageType: match.message_type,
        direction: match.direction,
        radio: match.radio,
        address: match.address,
      });
      setViewEventDrawerOpen(true);
    }
  }, [eventIdParam, networkLogsQuery.data?.items]);

  const handleConfirmDeleteRadioEvents = async () => {
    if (!accessToken) return;
    try {
      await clearRadioEvents(accessToken);
      setNetworkClearModalOpen(false);
      showSnackbar("All radio events cleared successfully.", "success");
      networkLogsQuery.refetch();
    } catch (error: unknown) {
      setNetworkClearModalOpen(false);
      showSnackbar(`Failed to clear radio events: ${String(error)}`, "error");
    }
  };

  const networkColumns: GridColDef<APIRadioEvent>[] = useMemo(() => {
    return [
      {
        field: "timestamp_dt",
        headerName: "Timestamp",
        type: "dateTime",
        flex: 1,
        minWidth: 180,
        sortable: false,
        filterable: false,
        renderCell: (p) => (p.value ? p.value.toLocaleString() : ""),
      },
      {
        field: "radio",
        headerName: "Radio",
        flex: 1,
        minWidth: 160,
        sortable: false,
        filterable: false,
        renderCell: (p) => {
          const radioName = p.row.radio;
          const address = p.row.address || "";
          if (!radioName) {
            return (
              <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
                {address || "—"}
              </Typography>
            );
          }
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
                to={`/radios/${encodeURIComponent(radioName)}`}
                style={{ textDecoration: "none" }}
                onClick={(e: React.MouseEvent) => e.stopPropagation()}
              >
                <Typography
                  variant="body2"
                  sx={{
                    color: theme.palette.link,
                    textDecoration: "underline",
                    "&:hover": { textDecoration: "underline" },
                  }}
                >
                  {radioName}
                  {address ? ` (${address})` : ""}
                </Typography>
              </Link>
            </Box>
          );
        },
      },
      {
        field: "message_type",
        headerName: "Message Type",
        flex: 1,
        minWidth: 220,
        sortable: false,
        filterable: false,
      },
      {
        field: "direction",
        headerName: "Direction",
        width: 120,
        align: "center",
        headerAlign: "center",
        sortable: false,
        filterable: false,
        renderCell: (p) => <DirectionCell value={p.row.direction} />,
      },
    ];
  }, [theme]);

  const handleRowClick = useCallback((params: GridRowParams<APIRadioEvent>) => {
    const r = params.row;
    setSelectionModel(makeSelection([params.id]));
    setSelectedRow({
      id: String(r.id),
      timestamp: r.timestamp,
      protocol: r.protocol,
      messageType: r.message_type,
      direction: r.direction,
      radio: r.radio,
      address: r.address,
    });
    setViewEventDrawerOpen(true);
  }, []);

  const subDescription =
    "Review NGAP messages between Ella Core and 5G radios. These logs are useful for auditing and troubleshooting purposes.";

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
        pt: 3,
        pb: 0,
        width: "100%",
      }}
    >
      <PanelGroup
        direction="horizontal"
        style={{ width: "100%", height: "100%", overflow: "hidden" }}
      >
        <Panel minSize={20}>
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

              {/* Filters + actions row */}
              <Box
                sx={{
                  display: "flex",
                  flexDirection: { xs: "column", sm: "row" },
                  flexWrap: "wrap",
                  gap: 2,
                  alignItems: { xs: "flex-start", sm: "center" },
                  flexShrink: 0,
                }}
              >
                <TextField
                  select
                  label="Radio"
                  value={radioFilter}
                  onChange={(e) => setRadioFilter(e.target.value)}
                  size="small"
                  sx={{ minWidth: 180 }}
                >
                  <MenuItem value="">All radios</MenuItem>
                  {radioOptions.map((r) => (
                    <MenuItem key={r.name} value={r.name}>
                      {r.name} ({r.address})
                    </MenuItem>
                  ))}
                </TextField>
                <TextField
                  select
                  label="Direction"
                  value={directionFilter}
                  onChange={(e) => setDirectionFilter(e.target.value)}
                  size="small"
                  sx={{ minWidth: 160 }}
                >
                  <MenuItem value="">All</MenuItem>
                  <MenuItem value="inbound">
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                      Inbound
                      <WestIcon
                        fontSize="small"
                        sx={{ color: theme.palette.success.main }}
                      />
                    </Box>
                  </MenuItem>
                  <MenuItem value="outbound">
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                      Outbound
                      <EastIcon
                        fontSize="small"
                        sx={{ color: theme.palette.info.main }}
                      />
                    </Box>
                  </MenuItem>
                </TextField>
                <TextField
                  select
                  label="Message Type"
                  value={messageTypeFilter}
                  onChange={(e) => setMessageTypeFilter(e.target.value)}
                  size="small"
                  sx={{ minWidth: 220 }}
                >
                  <MenuItem value="">All</MenuItem>
                  {NGAP_MESSAGE_TYPES.map((mt) => (
                    <MenuItem key={mt} value={mt}>
                      {mt}
                    </MenuItem>
                  ))}
                </TextField>

                <Box sx={{ flex: 1 }} />

                {canEdit && (
                  <Button
                    variant="outlined"
                    color="error"
                    size="small"
                    startIcon={<DeleteOutlineIcon />}
                    onClick={() => setNetworkClearModalOpen(true)}
                  >
                    Clear All
                  </Button>
                )}
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
                >
                  Retention: <strong>{retentionQuery.data?.days ?? "…"}</strong>{" "}
                  days
                  {canEdit && (
                    <IconButton
                      aria-label="edit radio event retention"
                      size="small"
                      color="primary"
                      onClick={() => setNetworkEditModalOpen(true)}
                    >
                      <EditIcon fontSize="small" />
                    </IconButton>
                  )}
                </Typography>
              </Box>

              <Box sx={{ flex: 1, minHeight: 0 }}>
                <DataGrid<APIRadioEvent>
                  rows={networkRows}
                  columns={networkColumns}
                  getRowId={(row) => row.id}
                  loading={
                    networkLogsQuery.isLoading ||
                    networkLogsQuery.isPlaceholderData
                  }
                  paginationMode="server"
                  rowCount={subRowCount}
                  paginationModel={paginationModel}
                  onPaginationModelChange={setPaginationModel}
                  disableColumnMenu
                  pageSizeOptions={[10, 25, 50, 100]}
                  onRowClick={handleRowClick}
                  rowSelectionModel={selectionModel}
                  disableRowSelectionOnClick
                  onRowSelectionModelChange={(model) =>
                    setSelectionModel(model)
                  }
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

              <EventDetails open={viewEventDrawerOpen} log={selectedRow} />
            </Box>
          </Panel>
        )}
      </PanelGroup>

      {/* Modals */}
      <EditRadioEventRetentionPolicyModal
        open={isNetworkEditModalOpen}
        onClose={() => setNetworkEditModalOpen(false)}
        onSuccess={() => {
          retentionQuery.refetch();
          showSnackbar("Retention policy updated successfully.", "success");
        }}
        initialDays={retentionQuery.data?.days || 7}
      />
      <DeleteConfirmationModal
        title="Clear All Network Logs"
        description="Are you sure you want to clear all radio events? This action cannot be undone."
        open={isNetworkClearModalOpen}
        onClose={() => setNetworkClearModalOpen(false)}
        onConfirm={handleConfirmDeleteRadioEvents}
      />
    </Box>
  );
};

// ----------------------------- Radios page -----------------------------

const Radio = () => {
  const { role, accessToken, authReady } = useAuth();
  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));

  const [searchParams, setSearchParams] = useSearchParams();

  const tab = (searchParams.get("tab") as TabKey) || "radios";

  const handleTabChange = (_: React.SyntheticEvent, newValue: TabKey) => {
    setSearchParams({ tab: newValue }, { replace: true });
  };

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const { data, isLoading } = useQuery<ListRadiosResponse>({
    queryKey: ["radios", paginationModel.page, paginationModel.pageSize],
    queryFn: async () => {
      const pageOneBased = paginationModel.page + 1;
      return listRadios(
        accessToken || "",
        pageOneBased,
        paginationModel.pageSize,
      );
    },
    enabled: !!accessToken && tab === "radios",
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const rows: APIRadio[] = data?.items ?? [];
  const rowCount: number = data?.total_count ?? 0;

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: theme.palette.backgroundSubtle } },
      }),
    [theme],
  );

  const columns: GridColDef<APIRadio>[] = useMemo(
    () => [
      {
        field: "name",
        headerName: "Name",
        flex: 1,
        minWidth: 200,
        renderCell: (params) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Link
              to={`/radios/${encodeURIComponent(params.row.name)}`}
              style={{ textDecoration: "none" }}
              onClick={(e: React.MouseEvent) => e.stopPropagation()}
            >
              <Typography
                variant="body2"
                sx={{
                  color: theme.palette.link,
                  textDecoration: "underline",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                {params.row.name}
              </Typography>
            </Link>
          </Box>
        ),
      },
      { field: "id", headerName: "ID", flex: 0.6, minWidth: 160 },
      {
        field: "ran_node_type",
        headerName: "Type",
        width: 120,
        renderCell: (params) => {
          const t = params.row.ran_node_type;
          const color =
            t === "gNB"
              ? "primary"
              : t === "ng-eNB"
                ? "secondary"
                : t === "N3IWF"
                  ? "warning"
                  : "default";
          return (
            <Chip size="small" label={t} color={color} variant="outlined" />
          );
        },
      },
      { field: "address", headerName: "Address", flex: 1, minWidth: 240 },
    ],
    [theme],
  );

  const descriptionText =
    "View connected radios and their network locations. Radios will automatically appear here once connected.";

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
      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: PAGE_PAD }}>
        <Tabs
          value={tab}
          onChange={handleTabChange}
          aria-label="Radios tabs"
          sx={{ borderBottom: 1, borderColor: "divider", mt: 2 }}
        >
          <Tab value="radios" label="Radios" />
          <Tab value="events" label="Events" />
        </Tabs>
      </Box>

      {/* ----------------- Radios Tab ----------------- */}
      {tab === "radios" && (
        <>
          {isLoading && rowCount === 0 ? (
            <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
              <CircularProgress />
            </Box>
          ) : rowCount === 0 ? (
            <EmptyState
              primaryText="No radio found."
              secondaryText="Connected radios will automatically appear here."
              extraContent={
                <Typography variant="body1" color="text.secondary">
                  {descriptionText}
                </Typography>
              }
              button={false}
            />
          ) : (
            <>
              <Box
                sx={{
                  width: "100%",
                  maxWidth: MAX_WIDTH,
                  px: PAGE_PAD,
                  mb: 3,
                  display: "flex",
                  flexDirection: "column",
                  gap: 2,
                  mt: 2,
                }}
              >
                <Typography variant="h4">Radios ({rowCount})</Typography>

                <Typography variant="body1" color="text.secondary">
                  {descriptionText}
                </Typography>
              </Box>

              <Box
                sx={{
                  width: "100%",
                  maxWidth: MAX_WIDTH,
                  px: PAGE_PAD,
                }}
              >
                <ThemeProvider theme={gridTheme}>
                  <DataGrid<APIRadio>
                    rows={rows}
                    columns={columns}
                    getRowId={(row) => row.address}
                    paginationMode="server"
                    rowCount={rowCount}
                    paginationModel={paginationModel}
                    onPaginationModelChange={setPaginationModel}
                    pageSizeOptions={[10, 25, 50, 100]}
                    disableColumnMenu
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
                      },
                      "& .MuiDataGrid-footerContainer": {
                        borderTop: "1px solid",
                        borderColor: "divider",
                      },
                    }}
                  />
                </ThemeProvider>
              </Box>
            </>
          )}
        </>
      )}

      {/* ----------------- Events Tab ----------------- */}
      {tab === "events" && <EventsTab />}
    </Box>
  );
};

export default Radio;
