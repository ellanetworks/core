import React, { useMemo, useState, useEffect, useRef } from "react";
import {
  Box,
  Button,
  Typography,
  CircularProgress,
  Alert,
  Collapse,
  TextField,
  IconButton,
} from "@mui/material";
import { Edit as EditIcon } from "@mui/icons-material";
import DeleteOutlineIcon from "@mui/icons-material/DeleteOutline";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import {
  DataGrid,
  type GridColDef,
  type GridPaginationModel,
  type GridSortModel,
} from "@mui/x-data-grid";
import {
  listFlowReports,
  clearFlowReports,
  getFlowReportsRetentionPolicy,
  type FlowReport,
  type ListFlowReportsResponse,
  type FlowReportsRetentionPolicy,
} from "@/queries/flow_reports";
import { useAuth } from "@/contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";
import EditFlowReportsRetentionPolicyModal from "@/components/EditFlowReportsRetentionPolicyModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";

const MAX_WIDTH = 1400;

// Complete IANA IPv4 protocol number table (RFC 5237 / IANA registry)
const PROTOCOL_NAMES: Record<number, string> = {
  0: "HOPOPT",
  1: "ICMP",
  2: "IGMP",
  3: "GGP",
  4: "IPv4",
  5: "ST",
  6: "TCP",
  7: "CBT",
  8: "EGP",
  9: "IGP",
  10: "BBN-RCC-MON",
  11: "NVP-II",
  12: "PUP",
  13: "ARGUS",
  14: "EMCON",
  15: "XNET",
  16: "CHAOS",
  17: "UDP",
  18: "MUX",
  19: "DCN-MEAS",
  20: "HMP",
  21: "PRM",
  22: "XNS-IDP",
  23: "TRUNK-1",
  24: "TRUNK-2",
  25: "LEAF-1",
  26: "LEAF-2",
  27: "RDP",
  28: "IRTP",
  29: "ISO-TP4",
  30: "NETBLT",
  31: "MFE-NSP",
  32: "MERIT-INP",
  33: "DCCP",
  34: "3PC",
  35: "IDPR",
  36: "XTP",
  37: "DDP",
  38: "IDPR-CMTP",
  39: "TP++",
  40: "IL",
  41: "IPv6",
  42: "SDRP",
  43: "IPv6-Route",
  44: "IPv6-Frag",
  45: "IDRP",
  46: "RSVP",
  47: "GRE",
  48: "DSR",
  49: "BNA",
  50: "ESP",
  51: "AH",
  52: "I-NLSP",
  53: "SWIPE",
  54: "NARP",
  55: "Min-IPv4",
  56: "TLSP",
  57: "SKIP",
  58: "IPv6-ICMP",
  59: "IPv6-NoNxt",
  60: "IPv6-Opts",
  62: "CFTP",
  64: "SAT-EXPAK",
  65: "KRYPTOLAN",
  66: "RVD",
  67: "IPPC",
  69: "SAT-MON",
  70: "VISA",
  71: "IPCV",
  72: "CPNX",
  73: "CPHB",
  74: "WSN",
  75: "PVP",
  76: "BR-SAT-MON",
  77: "SUN-ND",
  78: "WB-MON",
  79: "WB-EXPAK",
  80: "ISO-IP",
  81: "VMTP",
  82: "SECURE-VMTP",
  83: "VINES",
  84: "IPTM",
  85: "NSFNET-IGP",
  86: "DGP",
  87: "TCF",
  88: "EIGRP",
  89: "OSPFIGP",
  90: "Sprite-RPC",
  91: "LARP",
  92: "MTP",
  93: "AX.25",
  94: "IPIP",
  95: "MICP",
  96: "SCC-SP",
  97: "ETHERIP",
  98: "ENCAP",
  100: "GMTP",
  101: "IFMP",
  102: "PNNI",
  103: "PIM",
  104: "ARIS",
  105: "SCPS",
  106: "QNX",
  107: "A/N",
  108: "IPComp",
  109: "SNP",
  110: "Compaq-Peer",
  111: "IPX-in-IP",
  112: "VRRP",
  113: "PGM",
  115: "L2TP",
  116: "DDX",
  117: "IATP",
  118: "STP",
  119: "SRP",
  120: "UTI",
  121: "SMP",
  122: "SM",
  123: "PTP",
  124: "ISIS",
  125: "FIRE",
  126: "CRTP",
  127: "CRUDP",
  128: "SSCOPMCE",
  129: "IPLT",
  130: "SPS",
  131: "PIPE",
  132: "SCTP",
  133: "FC",
  134: "RSVP-E2E-IGNORE",
  135: "Mobility",
  136: "UDPLite",
  137: "MPLS-in-IP",
  138: "manet",
  139: "HIP",
  140: "Shim6",
  141: "WESP",
  142: "ROHC",
  143: "Ethernet",
  144: "AGGFRAG",
  145: "NSH",
};

// Reverse map: uppercase name → protocol number (for filter input parsing)
const PROTOCOL_NUMBER_BY_NAME: Record<string, number> = Object.fromEntries(
  Object.entries(PROTOCOL_NAMES).map(([num, name]) => [
    name.toUpperCase(),
    Number(num),
  ]),
);

const formatProtocol = (value: number): string =>
  PROTOCOL_NAMES[value] ?? String(value);

/**
 * Resolve a user-typed protocol filter string to a numeric string suitable
 * for the API query parameter.  Returns an empty string if the input is empty
 * or cannot be resolved.
 */
const resolveProtocolFilter = (input: string): string => {
  const trimmed = input.trim();
  if (trimmed === "") return "";
  // If it's already a plain number, use it directly
  if (/^\d+$/.test(trimmed)) return trimmed;
  // Otherwise look it up by name (case-insensitive)
  const num = PROTOCOL_NUMBER_BY_NAME[trimmed.toUpperCase()];
  return num !== undefined ? String(num) : "";
};

type DataUnit = "B" | "KB" | "MB" | "GB" | "TB";

const UNIT_FACTORS: Record<DataUnit, number> = {
  B: 1,
  KB: 1024,
  MB: 1024 ** 2,
  GB: 1024 ** 3,
  TB: 1024 ** 4,
};

const chooseUnitFromMax = (maxBytes: number): DataUnit => {
  if (maxBytes >= UNIT_FACTORS.TB) return "TB";
  if (maxBytes >= UNIT_FACTORS.GB) return "GB";
  if (maxBytes >= UNIT_FACTORS.MB) return "MB";
  if (maxBytes >= UNIT_FACTORS.KB) return "KB";
  return "B";
};

const formatBytesAutoUnit = (bytes: number): string => {
  if (!Number.isFinite(bytes)) return "";
  const unit = chooseUnitFromMax(Math.abs(bytes));
  const factor = UNIT_FACTORS[unit];
  const value = bytes / factor;
  const decimals = value >= 100 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(decimals)} ${unit}`;
};

const getDefaultDateRange = () => {
  const today = new Date();
  const sevenDaysAgo = new Date();
  sevenDaysAgo.setDate(today.getDate() - 6);
  const format = (d: Date) => d.toISOString().slice(0, 10);
  return { startDate: format(sevenDaysAgo), endDate: format(today) };
};

const FlowReports: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin";

  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: "#F5F5F5" } },
      }),
    [theme],
  );

  const [{ startDate, endDate }, setDateRange] = useState(getDefaultDateRange);

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [sortModel, setSortModel] = useState<GridSortModel>([]);

  // Filter state — debounced before being sent to the query
  const [subscriberFilter, setSubscriberFilter] = useState("");
  const [protocolFilter, setProtocolFilter] = useState("");
  const [sourceIpFilter, setSourceIpFilter] = useState("");
  const [destinationIpFilter, setDestinationIpFilter] = useState("");

  // Debounced (applied) filter values
  const [appliedSubscriber, setAppliedSubscriber] = useState("");
  const [appliedProtocol, setAppliedProtocol] = useState("");
  const [appliedSourceIp, setAppliedSourceIp] = useState("");
  const [appliedDestinationIp, setAppliedDestinationIp] = useState("");

  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const scheduleDebounce = (
    setter: React.Dispatch<React.SetStateAction<string>>,
    value: string,
  ) => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setter(value);
      setPaginationModel((prev) => ({ ...prev, page: 0 }));
    }, 400);
  };

  const scheduleProtocolDebounce = (raw: string) => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    debounceRef.current = setTimeout(() => {
      setAppliedProtocol(resolveProtocolFilter(raw));
      setPaginationModel((prev) => ({ ...prev, page: 0 }));
    }, 400);
  };

  // Reset page when date range changes
  useEffect(() => {
    setPaginationModel((prev) => ({ ...prev, page: 0 }));
  }, [startDate, endDate]);

  const [isEditRetentionModalOpen, setEditRetentionModalOpen] = useState(false);
  const [isClearModalOpen, setClearModalOpen] = useState(false);

  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });

  const pageOneBased = paginationModel.page + 1;

  const activeFilters = useMemo(() => {
    const f: Record<string, string> = {
      start: startDate,
      end: endDate,
    };
    if (appliedSubscriber) f.subscriber_id = appliedSubscriber;
    if (appliedProtocol) f.protocol = appliedProtocol;
    if (appliedSourceIp) f.source_ip = appliedSourceIp;
    if (appliedDestinationIp) f.destination_ip = appliedDestinationIp;
    return f;
  }, [
    startDate,
    endDate,
    appliedSubscriber,
    appliedProtocol,
    appliedSourceIp,
    appliedDestinationIp,
  ]);

  const { data: retentionPolicy, refetch: refetchRetentionPolicy } =
    useQuery<FlowReportsRetentionPolicy>({
      queryKey: ["flowReportsRetentionPolicy"],
      queryFn: () => getFlowReportsRetentionPolicy(accessToken || ""),
      enabled: authReady && !!accessToken,
    });

  const {
    data,
    isLoading,
    refetch: refetchFlowReports,
  } = useQuery<ListFlowReportsResponse>({
    queryKey: [
      "flowReports",
      pageOneBased,
      paginationModel.pageSize,
      activeFilters,
    ],
    queryFn: () =>
      listFlowReports(
        accessToken || "",
        pageOneBased,
        paginationModel.pageSize,
        activeFilters,
      ),
    enabled: authReady && !!accessToken,
    placeholderData: (prev) => prev,
    refetchInterval: 5000,
  });

  const rows: FlowReport[] = data?.items ?? [];
  const rowCount = data?.total_count ?? 0;

  const columns: GridColDef<FlowReport>[] = useMemo(
    () => [
      {
        field: "subscriber_id",
        headerName: "Subscriber",
        flex: 1,
        minWidth: 160,
      },
      {
        field: "source_ip",
        headerName: "Source IP",
        flex: 1,
        minWidth: 140,
      },
      {
        field: "source_port",
        headerName: "Src Port",
        type: "number",
        width: 100,
        renderCell: (params) => {
          const proto = (params.row as FlowReport).protocol;
          return proto === 6 || proto === 17 ? params.value : "";
        },
      },
      {
        field: "destination_ip",
        headerName: "Destination IP",
        flex: 1,
        minWidth: 140,
      },
      {
        field: "destination_port",
        headerName: "Dst Port",
        type: "number",
        width: 100,
        renderCell: (params) => {
          const proto = (params.row as FlowReport).protocol;
          return proto === 6 || proto === 17 ? params.value : "";
        },
      },
      {
        field: "protocol",
        headerName: "Protocol",
        width: 110,
        valueFormatter: (value: number) =>
          value == null ? "" : formatProtocol(value),
      },
      {
        field: "packets",
        headerName: "Packets",
        type: "number",
        width: 110,
        valueFormatter: (value: number) =>
          value == null ? "" : value.toLocaleString(),
      },
      {
        field: "bytes",
        headerName: "Bytes",
        type: "number",
        width: 120,
        valueFormatter: (value: number) =>
          value == null ? "" : formatBytesAutoUnit(value),
      },
      {
        field: "start_time",
        headerName: "Start",
        flex: 1,
        minWidth: 180,
        valueFormatter: (value: string) =>
          value ? new Date(value).toLocaleString() : "",
      },
      {
        field: "end_time",
        headerName: "End",
        flex: 1,
        minWidth: 180,
        valueFormatter: (value: string) =>
          value ? new Date(value).toLocaleString() : "",
      },
    ],
    [],
  );

  const handleConfirmClear = async () => {
    setClearModalOpen(false);
    if (!accessToken) return;
    try {
      await clearFlowReports(accessToken);
      await refetchFlowReports();
      setAlert({
        message: "All flow report data cleared successfully!",
        severity: "success",
      });
    } catch (error: unknown) {
      setAlert({
        message: `Failed to clear flow report data: ${
          error instanceof Error ? error.message : String(error)
        }`,
        severity: "error",
      });
    }
  };

  const isInitialLoading = isLoading && !data;

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
      {/* Alert banner */}
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

      {isInitialLoading ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : (
        <>
          {/* Header + filters */}
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
            <Typography variant="h4">Flow Reports</Typography>

            <Typography variant="body1" color="text.secondary">
              View individual network flows collected by the user plane. Each
              row represents a single flow between a subscriber and a remote
              endpoint.
            </Typography>

            {/* Filter row */}
            <Box
              sx={{
                display: "flex",
                flexDirection: { xs: "column", sm: "row" },
                gap: 2,
                alignItems: { xs: "flex-start", sm: "center" },
                justifyContent: "space-between",
                flexWrap: "wrap",
              }}
            >
              <Box
                sx={{
                  display: "flex",
                  flexDirection: { xs: "column", sm: "row" },
                  gap: 2,
                  alignItems: { xs: "flex-start", sm: "center" },
                  flexWrap: "wrap",
                }}
              >
                <TextField
                  label="Start date"
                  type="date"
                  value={startDate}
                  onChange={(e) =>
                    setDateRange((prev) => ({
                      ...prev,
                      startDate: e.target.value,
                    }))
                  }
                  InputLabelProps={{ shrink: true }}
                  size="small"
                />
                <TextField
                  label="End date"
                  type="date"
                  value={endDate}
                  onChange={(e) =>
                    setDateRange((prev) => ({
                      ...prev,
                      endDate: e.target.value,
                    }))
                  }
                  InputLabelProps={{ shrink: true }}
                  size="small"
                />
                <TextField
                  label="Subscriber ID"
                  value={subscriberFilter}
                  onChange={(e) => {
                    setSubscriberFilter(e.target.value);
                    scheduleDebounce(setAppliedSubscriber, e.target.value);
                  }}
                  size="small"
                  sx={{ minWidth: 160 }}
                />
                <TextField
                  label="Protocol"
                  value={protocolFilter}
                  onChange={(e) => {
                    setProtocolFilter(e.target.value);
                    scheduleProtocolDebounce(e.target.value);
                  }}
                  size="small"
                  sx={{ minWidth: 100 }}
                  placeholder="e.g. TCP or 6"
                />
                <TextField
                  label="Source IP"
                  value={sourceIpFilter}
                  onChange={(e) => {
                    setSourceIpFilter(e.target.value);
                    scheduleDebounce(setAppliedSourceIp, e.target.value);
                  }}
                  size="small"
                  sx={{ minWidth: 140 }}
                />
                <TextField
                  label="Destination IP"
                  value={destinationIpFilter}
                  onChange={(e) => {
                    setDestinationIpFilter(e.target.value);
                    scheduleDebounce(setAppliedDestinationIp, e.target.value);
                  }}
                  size="small"
                  sx={{ minWidth: 140 }}
                />
              </Box>

              {/* Retention controls */}
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  mt: { xs: 1, sm: 0 },
                  ml: { xs: 0, sm: "auto" },
                  flexShrink: 0,
                }}
              >
                {canEdit && (
                  <Button
                    variant="outlined"
                    color="error"
                    size="small"
                    startIcon={<DeleteOutlineIcon />}
                    onClick={() => setClearModalOpen(true)}
                    sx={{ flexShrink: 0 }}
                  >
                    Clear All
                  </Button>
                )}
                <Typography variant="body2" color="text.secondary">
                  Retention: <strong>{retentionPolicy?.days ?? "…"}</strong>{" "}
                  days
                </Typography>
                {canEdit && (
                  <IconButton
                    aria-label="edit retention"
                    size="small"
                    onClick={() => setEditRetentionModalOpen(true)}
                  >
                    <EditIcon fontSize="small" />
                  </IconButton>
                )}
              </Box>
            </Box>
          </Box>

          {/* Table */}
          <Box
            sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}
          >
            {rowCount === 0 && !isLoading ? (
              <EmptyState
                primaryText="No flow reports found"
                secondaryText="No flows match the current filters, or flow accounting has not recorded any data yet."
                button={false}
                buttonText=""
                onCreate={() => {}}
              />
            ) : (
              <ThemeProvider theme={gridTheme}>
                <DataGrid<FlowReport>
                  rows={rows}
                  columns={columns}
                  getRowId={(row) => row.id}
                  paginationMode="server"
                  rowCount={rowCount}
                  paginationModel={paginationModel}
                  onPaginationModelChange={setPaginationModel}
                  sortModel={sortModel}
                  onSortModelChange={setSortModel}
                  sortingMode="client"
                  disableColumnMenu
                  disableRowSelectionOnClick
                  pageSizeOptions={[10, 25, 50, 100]}
                  density={isSmDown ? "compact" : "standard"}
                  columnVisibilityModel={{}}
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
                    "& .MuiDataGrid-columnHeaderTitle": {
                      fontWeight: "bold",
                    },
                  }}
                />
              </ThemeProvider>
            )}
          </Box>
        </>
      )}

      <EditFlowReportsRetentionPolicyModal
        open={isEditRetentionModalOpen}
        onClose={() => setEditRetentionModalOpen(false)}
        onSuccess={() => {
          refetchRetentionPolicy();
          setAlert({
            message: "Retention policy updated!",
            severity: "success",
          });
        }}
        initialData={retentionPolicy || { days: 30 }}
      />
      <DeleteConfirmationModal
        title="Clear All Flow Report Data"
        description="Are you sure you want to clear all flow report data? This action cannot be undone."
        open={isClearModalOpen}
        onClose={() => setClearModalOpen(false)}
        onConfirm={handleConfirmClear}
      />
    </Box>
  );
};

export default FlowReports;
