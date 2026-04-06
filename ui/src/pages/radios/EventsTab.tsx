import React, {
  useState,
  useMemo,
  useEffect,
  useCallback,
  useRef,
} from "react";
import {
  Box,
  Button,
  Typography,
  Tooltip,
  IconButton,
  TextField,
  MenuItem,
} from "@mui/material";
import { ThemeProvider } from "@mui/material/styles";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useTheme } from "@mui/material/styles";
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
import PauseIcon from "@mui/icons-material/Pause";
import PlayArrowIcon from "@mui/icons-material/PlayArrow";
import { Edit as EditIcon } from "@mui/icons-material";

import {
  listRadios,
  type APIRadio,
  type ListRadiosResponse,
} from "@/queries/radios";
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
import { formatDateTime } from "@/utils/formatters";
import { useRadiosContext } from "./types";

// -------- Helpers & small components --------

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

const PANEL_DEFAULT_WIDTH = 825;
const PANEL_MIN_WIDTH = 350;
const PANEL_MAX_VW = 0.8;
const TOOLBAR_HEIGHT = 64;

// ------------------- Events tab content -------------------

export default function EventsTab() {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin";
  const theme = useTheme();
  const { gridTheme } = useRadiosContext();

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
  const [searchParams, setSearchParams] = useSearchParams();
  const radioParam = searchParams.get("radio") ?? "";
  const eventIdParam = searchParams.get("event");

  // Explicit filter state (replaces DataGrid built-in filter model)
  const [radioFilter, setRadioFilter] = useState(radioParam);
  const [directionFilter, setDirectionFilter] = useState("");
  const [messageTypeFilter, setMessageTypeFilter] = useState("");
  const [timestampFrom, setTimestampFrom] = useState("");
  const [timestampTo, setTimestampTo] = useState("");

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
    if (timestampFrom)
      params.timestamp_from = new Date(timestampFrom).toISOString();
    if (timestampTo) params.timestamp_to = new Date(timestampTo).toISOString();
    return params;
  }, [
    radioFilter,
    directionFilter,
    messageTypeFilter,
    timestampFrom,
    timestampTo,
  ]);

  const networkLogsQuery = useQuery<ListRadioEventsResponse>({
    queryKey: ["networkLogs", pageOneBased, perPage, filterParams],
    enabled: authReady && !!accessToken,
    refetchInterval: autoRefresh && visible ? 3000 : false,
    placeholderData: keepPreviousData,
    queryFn: () =>
      listRadioEvents(accessToken!, pageOneBased, perPage, filterParams),
  });

  const networkRows = networkLogsQuery.data?.items ?? [];

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
        field: "timestamp",
        headerName: "Timestamp",
        flex: 1,
        minWidth: 140,
        sortable: false,
        filterable: false,
        renderCell: (p) => {
          const ts = p.row.timestamp;
          return ts ? formatDateTime(ts, { seconds: true }) : "";
        },
      },
      {
        field: "radio",
        headerName: "Radio",
        flex: 1,
        minWidth: 120,
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
                  component="span"
                  variant="body2"
                  sx={{
                    color: theme.palette.link,
                    textDecoration: "underline",
                    "&:hover": { textDecoration: "underline" },
                  }}
                >
                  {radioName}
                </Typography>
              </Link>
              {address && (
                <Typography
                  component="span"
                  variant="body2"
                  sx={{ ml: 0.5, color: "text.secondary" }}
                >
                  ({address})
                </Typography>
              )}
            </Box>
          );
        },
      },
      {
        field: "message_type",
        headerName: "Message Type",
        flex: 1,
        minWidth: 160,
        sortable: false,
        filterable: false,
      },
      {
        field: "direction",
        headerName: "Direction",
        width: 110,
        align: "center",
        headerAlign: "center",
        sortable: false,
        filterable: false,
        renderCell: (p) => <DirectionCell value={p.row.direction} />,
      },
    ];
  }, [theme]);

  const handleRowClick = useCallback(
    (params: GridRowParams<APIRadioEvent>) => {
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
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.set("event", String(r.id));
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const subDescription =
    "Review NGAP messages between Ella Core and 5G radios. These logs are useful for auditing and troubleshooting purposes.";

  const closePanel = useCallback(() => {
    setViewEventDrawerOpen(false);
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.delete("event");
        return next;
      },
      { replace: true },
    );
  }, [setSearchParams]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closePanel();
    };
    if (viewEventDrawerOpen) window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [viewEventDrawerOpen, closePanel]);

  // --- Resize handle state ---
  const [panelWidth, setPanelWidth] = useState(PANEL_DEFAULT_WIDTH);
  const dragging = useRef(false);

  const onResizeMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    dragging.current = true;

    const onMouseMove = (ev: MouseEvent) => {
      if (!dragging.current) return;
      const maxPx = window.innerWidth * PANEL_MAX_VW;
      const next = window.innerWidth - ev.clientX;
      setPanelWidth(Math.max(PANEL_MIN_WIDTH, Math.min(maxPx, next)));
    };
    const onMouseUp = () => {
      dragging.current = false;
      document.removeEventListener("mousemove", onMouseMove);
      document.removeEventListener("mouseup", onMouseUp);
    };
    document.addEventListener("mousemove", onMouseMove);
    document.addEventListener("mouseup", onMouseUp);
  }, []);

  return (
    <Box sx={{ pt: 3, width: "100%" }}>
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          gap: 2,
        }}
      >
        <Box>
          <Typography variant="h4">Network Events</Typography>
          <Typography variant="body1" color="text.secondary">
            {subDescription}
          </Typography>
        </Box>

        {/* Filters row */}
        <Box
          sx={{
            display: "flex",
            flexWrap: "wrap",
            gap: 2,
            alignItems: "center",
          }}
        >
          <TextField
            select
            label="Radio"
            value={radioFilter}
            onChange={(e) => setRadioFilter(e.target.value)}
            size="small"
            sx={{ minWidth: 150 }}
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
            sx={{ minWidth: 150 }}
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
            sx={{ minWidth: 180 }}
          >
            <MenuItem value="">All</MenuItem>
            {NGAP_MESSAGE_TYPES.map((mt) => (
              <MenuItem key={mt} value={mt}>
                {mt}
              </MenuItem>
            ))}
          </TextField>
          <TextField
            label="From"
            type="datetime-local"
            value={timestampFrom}
            onChange={(e) => setTimestampFrom(e.target.value)}
            size="small"
            slotProps={{ inputLabel: { shrink: true } }}
            sx={{ minWidth: 200 }}
          />
          <TextField
            label="To"
            type="datetime-local"
            value={timestampTo}
            onChange={(e) => setTimestampTo(e.target.value)}
            size="small"
            slotProps={{ inputLabel: { shrink: true } }}
            sx={{ minWidth: 200 }}
          />
        </Box>

        {/* Actions row */}
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 2,
          }}
        >
          <Button
            variant={autoRefresh ? "outlined" : "contained"}
            size="small"
            startIcon={autoRefresh ? <PauseIcon /> : <PlayArrowIcon />}
            onClick={() => {
              setAutoRefresh((prev) => {
                if (!prev) networkLogsQuery.refetch();
                return !prev;
              });
            }}
          >
            {autoRefresh ? "Pause" : "Live"}
          </Button>
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
          <Box sx={{ flex: 1 }} />
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ display: "flex", alignItems: "center", gap: 0.5 }}
          >
            Retention: <strong>{retentionQuery.data?.days ?? "…"}</strong> days
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

        <ThemeProvider theme={gridTheme}>
          <DataGrid<APIRadioEvent>
            rows={networkRows}
            columns={networkColumns}
            getRowId={(row) => row.id}
            loading={
              networkLogsQuery.isLoading || networkLogsQuery.isPlaceholderData
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
            onRowSelectionModelChange={(model) => setSelectionModel(model)}
            density="compact"
            autoHeight
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
              "& .MuiDataGrid-row:hover": { cursor: "pointer" },
              "& .MuiDataGrid-row.Mui-selected": {
                backgroundColor: (t) => t.palette.action.selected,
                "&:hover": {
                  backgroundColor: (t) => t.palette.action.selected,
                },
                "& .MuiDataGrid-cell": { fontWeight: 500 },
                "&::before": { display: "none" },
              },
              "& .MuiDataGrid-cell:focus, & .MuiDataGrid-cell:focus-within": {
                outline: "none",
              },
              "& .MuiDataGrid-columnHeader:focus, & .MuiDataGrid-columnHeader:focus-within":
                { outline: "none" },
            }}
          />
        </ThemeProvider>
      </Box>

      {/* Fixed overlay detail panel */}
      <Box
        sx={{
          position: "fixed",
          top: TOOLBAR_HEIGHT,
          right: 0,
          bottom: 0,
          width: panelWidth,
          transform: viewEventDrawerOpen ? "translateX(0)" : "translateX(100%)",
          transition: dragging.current ? "none" : "transform 200ms ease-in-out",
          zIndex: (t) => t.zIndex.appBar - 1,
          bgcolor: "background.paper",
          boxShadow: viewEventDrawerOpen
            ? "-4px 0 16px rgba(0,0,0,0.12)"
            : "none",
          display: "flex",
          flexDirection: "row",
        }}
      >
        {/* Drag handle */}
        <Box
          onMouseDown={onResizeMouseDown}
          sx={{
            width: 12,
            flexShrink: 0,
            cursor: "ew-resize",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            "&:hover .resizeIcon": { opacity: 1 },
          }}
        >
          <DragIndicatorIcon
            className="resizeIcon"
            sx={{
              fontSize: 20,
              opacity: 0.5,
              transition: "opacity 120ms",
              color: "text.secondary",
            }}
          />
        </Box>

        {/* Panel content */}
        <Box
          sx={{
            flex: 1,
            minWidth: 0,
            display: "flex",
            flexDirection: "column",
            borderLeft: (t) => `1px solid ${t.palette.divider}`,
          }}
        >
          <Box
            sx={{
              px: 2,
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
            <IconButton aria-label="Close" onClick={closePanel} size="small">
              <CloseIcon />
            </IconButton>
          </Box>

          <EventDetails open={viewEventDrawerOpen} log={selectedRow} />
        </Box>
      </Box>

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
}
