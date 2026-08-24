// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState, useMemo, useEffect, useCallback } from "react";
import {
  Alert,
  Box,
  Button,
  Typography,
  Tooltip,
  IconButton,
  TextField,
  MenuItem,
  ListSubheader,
} from "@mui/material";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useTheme } from "@mui/material/styles";
import {
  type GridColDef,
  type GridRowParams,
  type GridRowId,
  type GridRowSelectionModel,
} from "@mui/x-data-grid";
import EntityGrid from "@/components/grid/EntityGrid";
import { useSearchParams, Link } from "react-router-dom";
import EastIcon from "@mui/icons-material/East";
import WestIcon from "@mui/icons-material/West";
import CloseIcon from "@mui/icons-material/Close";
import DragIndicatorIcon from "@mui/icons-material/DragIndicator";
import DeleteOutlineIcon from "@mui/icons-material/DeleteOutlined";
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
import QueryState from "@/components/QueryState";
import EmptyState from "@/components/EmptyState";

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
import ProtocolChip from "@/components/ProtocolChip";
import { formatDateTime } from "@/utils/formatters";
import { useFilteredPagination } from "@/hooks/useFilteredPagination";

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

// S1AP (4G) message names recorded by the MME (see internal/mme/s1ap_procedures.go).
const S1AP_MESSAGE_TYPES = [
  "DownlinkNASTransport",
  "ErrorIndication",
  "InitialContextSetupFailure",
  "InitialContextSetupRequest",
  "InitialContextSetupResponse",
  "InitialUEMessage",
  "Paging",
  "S1SetupFailure",
  "S1SetupRequest",
  "S1SetupResponse",
  "UECapabilityInfoIndication",
  "UEContextReleaseCommand",
  "UEContextReleaseComplete",
  "UEContextReleaseRequest",
  "UplinkNASTransport",
];

const MESSAGE_TYPES_BY_PROTOCOL: Record<string, string[]> = {
  NGAP: NGAP_MESSAGE_TYPES,
  S1AP: S1AP_MESSAGE_TYPES,
};

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

const toIsoInstant = (value: string): string => {
  if (!value) return "";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? "" : parsed.toISOString();
};

const TIMESTAMP_ERROR_ID = "radio-events-timestamp-error";

const PANEL_DEFAULT_WIDTH = 825;
const PANEL_MIN_WIDTH = 350;
const PANEL_MAX_VW = 0.8;
const TOOLBAR_HEIGHT = 64;

const makeSelection = (ids: GridRowId[] = []): GridRowSelectionModel => ({
  type: "include",
  ids: new Set<GridRowId>(ids),
});

export default function EventsTab() {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin";
  const theme = useTheme();

  const { showSnackbar } = useSnackbar();
  const [autoRefresh, setAutoRefresh] = useState(true);
  const visible = usePageVisible();

  const [isNetworkEditModalOpen, setNetworkEditModalOpen] = useState(false);
  const [isNetworkClearModalOpen, setNetworkClearModalOpen] = useState(false);

  const [searchParams, setSearchParams] = useSearchParams();
  const radioParam = searchParams.get("radio") ?? "";
  const eventIdParam = searchParams.get("event");

  const [radioFilter, setRadioFilter] = useState(radioParam);
  const [protocolFilter, setProtocolFilter] = useState("");
  const [directionFilter, setDirectionFilter] = useState("");
  const [messageTypeFilter, setMessageTypeFilter] = useState("");
  const [timestampFrom, setTimestampFrom] = useState("");
  const [timestampTo, setTimestampTo] = useState("");

  const messageTypeOptions = useMemo(
    () =>
      MESSAGE_TYPES_BY_PROTOCOL[protocolFilter] ?? [
        ...NGAP_MESSAGE_TYPES,
        ...S1AP_MESSAGE_TYPES,
      ],
    [protocolFilter],
  );

  const effectiveMessageType = messageTypeOptions.includes(messageTypeFilter)
    ? messageTypeFilter
    : "";

  const [prevRadioParam, setPrevRadioParam] = useState(radioParam);
  if (radioParam !== prevRadioParam) {
    setPrevRadioParam(radioParam);
    setRadioFilter(radioParam);
  }

  const radiosQuery = useQuery<ListRadiosResponse>({
    queryKey: ["radios-for-filter"],
    queryFn: () => listRadios(accessToken!, 1, 100),
    enabled: authReady && !!accessToken,
    refetchInterval: 10_000,
  });
  const radioOptions: APIRadio[] = useMemo(() => {
    const radios = radiosQuery.data?.items ?? [];
    return radioFilter && !radios.some((r) => r.name === radioFilter)
      ? [
          {
            name: radioFilter,
            id: radioFilter,
            address: "",
            type: "",
            supported_tais: [],
          },
          ...radios,
        ]
      : radios;
  }, [radiosQuery.data, radioFilter]);

  const retentionQuery = useQuery<RadioEventRetentionPolicy>({
    queryKey: ["networkLogRetention"],
    enabled: authReady && !!accessToken && !isNetworkEditModalOpen,
    queryFn: () => getRadioEventRetentionPolicy(accessToken!),
  });

  const timestampFromIso = toIsoInstant(timestampFrom);
  const timestampToIso = toIsoInstant(timestampTo);

  const timestampError =
    (timestampFrom && !timestampFromIso) || (timestampTo && !timestampToIso)
      ? "Enter a valid date and time."
      : timestampFromIso && timestampToIso && timestampFromIso > timestampToIso
        ? "The To timestamp must be on or after the From timestamp."
        : "";

  const filterParams = useMemo(() => {
    const params: Record<string, string> = {};
    if (radioFilter) params.radio = radioFilter;
    if (protocolFilter) params.protocol = protocolFilter;
    if (directionFilter) params.direction = directionFilter;
    if (effectiveMessageType) params.message_type = effectiveMessageType;
    if (timestampFromIso) params.timestamp_from = timestampFromIso;
    if (timestampToIso) params.timestamp_to = timestampToIso;
    return params;
  }, [
    radioFilter,
    protocolFilter,
    directionFilter,
    effectiveMessageType,
    timestampFromIso,
    timestampToIso,
  ]);

  const [paginationModel, setPaginationModel] =
    useFilteredPagination(filterParams);
  const pageOneBased = paginationModel.page + 1;
  const perPage = paginationModel.pageSize;

  const networkLogsQuery = useQuery<ListRadioEventsResponse>({
    queryKey: ["networkLogs", pageOneBased, perPage, filterParams],
    enabled: authReady && !!accessToken && !timestampError,
    refetchInterval: autoRefresh && visible ? 3000 : false,
    placeholderData: keepPreviousData,
    queryFn: () =>
      listRadioEvents(accessToken!, pageOneBased, perPage, filterParams),
  });

  const networkRows = networkLogsQuery.data?.items ?? [];

  const subRowCount = networkLogsQuery.data?.total_count ?? 0;

  const hasActiveFilters = Object.keys(filterParams).length > 0;

  const viewEventDrawerOpen = !!eventIdParam;

  const selectionModel = useMemo(
    () => makeSelection(eventIdParam ? [Number(eventIdParam)] : []),
    [eventIdParam],
  );

  const eventRow = useMemo<LogRow | null>(() => {
    if (!eventIdParam) return null;
    const match = networkLogsQuery.data?.items?.find(
      (r) => r.id === Number(eventIdParam),
    );
    return match
      ? {
          id: String(match.id),
          timestamp: match.timestamp,
          protocol: match.protocol,
          messageType: match.message_type,
          direction: match.direction,
          radio: match.radio,
          address: match.address,
        }
      : null;
  }, [eventIdParam, networkLogsQuery.data?.items]);

  // Held past the param being cleared so the panel keeps its contents while it slides out.
  const [selectedRow, setSelectedRow] = useState<LogRow | null>(eventRow);
  if (eventRow && eventRow.id !== selectedRow?.id) {
    setSelectedRow(eventRow);
  }

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
            return <Typography variant="body2">{address || "—"}</Typography>;
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
        field: "protocol",
        headerName: "Protocol",
        width: 110,
        sortable: false,
        filterable: false,
        renderCell: (p) => <ProtocolChip protocol={p.row.protocol} />,
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
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          next.set("event", String(params.row.id));
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const subDescription =
    "Review NGAP (5G) and S1AP (4G) control-plane messages exchanged between Ella Core and connected radios. These logs are useful for auditing and troubleshooting purposes.";

  const handleSelectionChange = useCallback(
    (model: GridRowSelectionModel) => {
      const [id] = [...model.ids];
      setSearchParams(
        (prev) => {
          const next = new URLSearchParams(prev);
          if (id == null) next.delete("event");
          else next.set("event", String(id));
          return next;
        },
        { replace: true },
      );
    },
    [setSearchParams],
  );

  const closePanel = useCallback(() => {
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

  const [panelWidth, setPanelWidth] = useState(PANEL_DEFAULT_WIDTH);
  const [dragging, setDragging] = useState(false);

  const onResizeMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    setDragging(true);

    const onMouseMove = (ev: MouseEvent) => {
      const maxPx = window.innerWidth * PANEL_MAX_VW;
      const next = window.innerWidth - ev.clientX;
      setPanelWidth(Math.max(PANEL_MIN_WIDTH, Math.min(maxPx, next)));
    };
    const onMouseUp = () => {
      setDragging(false);
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
          <Typography variant="body1" color="textSecondary">
            {subDescription}
          </Typography>
        </Box>

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
            label="Protocol"
            value={protocolFilter}
            onChange={(e) => setProtocolFilter(e.target.value)}
            size="small"
            sx={{ minWidth: 150 }}
          >
            <MenuItem value="">All</MenuItem>
            <MenuItem value="NGAP">NGAP (5G)</MenuItem>
            <MenuItem value="S1AP">S1AP (4G)</MenuItem>
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
            value={effectiveMessageType}
            onChange={(e) => setMessageTypeFilter(e.target.value)}
            size="small"
            sx={{ minWidth: 180 }}
          >
            <MenuItem value="">All</MenuItem>
            {protocolFilter
              ? messageTypeOptions.map((mt) => (
                  <MenuItem key={mt} value={mt}>
                    {mt}
                  </MenuItem>
                ))
              : [
                  <ListSubheader key="ngap-header">NGAP (5G)</ListSubheader>,
                  ...NGAP_MESSAGE_TYPES.map((mt) => (
                    <MenuItem key={`ngap-${mt}`} value={mt}>
                      {mt}
                    </MenuItem>
                  )),
                  <ListSubheader key="s1ap-header">S1AP (4G)</ListSubheader>,
                  ...S1AP_MESSAGE_TYPES.map((mt) => (
                    <MenuItem key={`s1ap-${mt}`} value={mt}>
                      {mt}
                    </MenuItem>
                  )),
                ]}
          </TextField>
          <TextField
            label="From"
            type="datetime-local"
            value={timestampFrom}
            onChange={(e) => setTimestampFrom(e.target.value)}
            error={!!timestampError}
            size="small"
            slotProps={{
              inputLabel: { shrink: true },
              htmlInput: {
                "aria-describedby": timestampError
                  ? TIMESTAMP_ERROR_ID
                  : undefined,
              },
            }}
            sx={{ minWidth: 200 }}
          />
          <TextField
            label="To"
            type="datetime-local"
            value={timestampTo}
            onChange={(e) => setTimestampTo(e.target.value)}
            error={!!timestampError}
            size="small"
            slotProps={{
              inputLabel: { shrink: true },
              htmlInput: {
                min: timestampFrom || undefined,
                "aria-describedby": timestampError
                  ? TIMESTAMP_ERROR_ID
                  : undefined,
              },
            }}
            sx={{ minWidth: 200 }}
          />
        </Box>

        {timestampError && (
          <Alert
            id={TIMESTAMP_ERROR_ID}
            severity="error"
            sx={{ alignSelf: "flex-start" }}
          >
            {timestampError}
          </Alert>
        )}

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
            color="textSecondary"
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

        {timestampError ? null : (
          <QueryState
            query={networkLogsQuery}
            resource="radio events"
            isEmpty={(data) => (data.total_count ?? 0) === 0}
            filtered={hasActiveFilters}
            noResults={
              <EmptyState
                primaryText="No radio events match the selected filters"
                secondaryText="Try clearing the radio, protocol, direction, message type, or time filters."
              />
            }
            empty={
              <EmptyState
                primaryText="No radio events yet"
                secondaryText="Signalling exchanged with connected radios will appear here."
              />
            }
          >
            {() => (
              <EntityGrid<APIRadioEvent>
                variant="log"
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
                onRowClick={handleRowClick}
                rowSelectionModel={selectionModel}
                onRowSelectionModelChange={handleSelectionChange}
                sx={{
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
                    {
                      outline: "none",
                    },
                  "& .MuiDataGrid-columnHeader:focus, & .MuiDataGrid-columnHeader:focus-within":
                    { outline: "none" },
                }}
              />
            )}
          </QueryState>
        )}
      </Box>

      <Box
        sx={{
          position: "fixed",
          top: TOOLBAR_HEIGHT,
          right: 0,
          bottom: 0,
          width: panelWidth,
          transform: viewEventDrawerOpen ? "translateX(0)" : "translateX(100%)",
          transition: dragging ? "none" : "transform 200ms ease-in-out",
          zIndex: (t) => t.zIndex.appBar - 1,
          bgcolor: "background.paper",
          boxShadow: viewEventDrawerOpen ? 8 : "none",
          display: "flex",
          flexDirection: "row",
        }}
      >
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
            <Box
              sx={{
                flex: 1,
                minWidth: 0,
                display: "flex",
                alignItems: "center",
                gap: 1,
              }}
            >
              <Typography variant="h6" noWrap>
                {selectedRow?.messageType ?? "Event details"}
              </Typography>
              {selectedRow?.protocol && (
                <ProtocolChip protocol={selectedRow.protocol} />
              )}
            </Box>
            <IconButton aria-label="Close" onClick={closePanel} size="small">
              <CloseIcon />
            </IconButton>
          </Box>

          <EventDetails open={viewEventDrawerOpen} log={selectedRow} />
        </Box>
      </Box>

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
