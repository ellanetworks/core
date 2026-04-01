import React, { useState, useMemo } from "react";
import {
  Alert,
  Box,
  Typography,
  Button,
  CircularProgress,
  FormControlLabel,
  Chip,
  Stack,
  Tabs,
  Tab,
  Switch,
  IconButton,
  Tooltip,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  Delete as DeleteIcon,
  Edit as EditIcon,
  Visibility as ViewIcon,
} from "@mui/icons-material";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useSearchParams, Link } from "react-router-dom";

// Data Networks
import {
  listDataNetworks,
  type ListDataNetworksResponse,
  type APIDataNetwork,
} from "@/queries/data_networks";
import CreateDataNetworkModal from "@/components/CreateDataNetworkModal";

// Routes
import {
  listRoutes,
  deleteRoute,
  type ListRoutesResponse,
  type APIRoute,
} from "@/queries/routes";
import CreateRouteModal from "@/components/CreateRouteModal";

// NAT
import { getNATInfo, updateNATInfo, type NatInfo } from "@/queries/nat";

// BGP
import {
  getBGPSettings,
  updateBGPSettings,
  listBGPPeers,
  deleteBGPPeer,
  getBGPAdvertisedRoutes,
  getBGPLearnedRoutes,
  type BGPSettings,
  type ListBGPPeersResponse,
  type BGPAdvertisedRoutesResponse,
  type BGPLearnedRoutesResponse,
  type BGPPeer as APIBGPPeer,
  type BGPAdvertisedRoute,
  type BGPLearnedRoute,
  type BGPImportPrefix,
  type RejectedPrefix,
} from "@/queries/bgp";
import CreateBGPPeerModal from "@/components/CreateBGPPeerModal";
import EditBGPPeerModal from "@/components/EditBGPPeerModal";
import ViewBGPPeerModal from "@/components/ViewBGPPeerModal";
import EditBGPSettingsModal from "@/components/EditBGPSettingsModal";

// Flow Accounting
import {
  getFlowAccountingInfo,
  updateFlowAccountingInfo,
  type FlowAccountingInfo,
} from "@/queries/flow_accounting";

// Interfaces
import {
  getInterfaces,
  type InterfacesInfo,
  type VlanInfo,
} from "@/queries/interfaces";
import EditInterfaceN3Modal from "@/components/EditInterfaceN3Modal";

// Shared UI
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";

import {
  DataGrid,
  type GridColDef,
  GridActionsCellItem,
  type GridRenderCellParams,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

type TabKey =
  | "data-networks"
  | "interfaces"
  | "routes"
  | "nat"
  | "bgp"
  | "flow-accounting";

export default function NetworkingPage() {
  const { role, accessToken } = useAuth();
  const canEdit = role === "Admin" || role === "Network Manager";

  const [searchParams, setSearchParams] = useSearchParams();

  // Read initial tab from URL, fallback to default
  const initialTabFromUrl =
    (searchParams.get("tab") as TabKey) || "data-networks";

  const [tab, setTab] = useState<TabKey>(initialTabFromUrl);

  // ---------------- Snackbar ----------------
  const { showSnackbar } = useSnackbar();

  // ====================== Data Networks ======================
  const [dnPagination, setDnPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const {
    data: dnPage,
    isLoading: dnLoading,
    refetch: refetchDataNetworks,
  } = useQuery<ListDataNetworksResponse>({
    queryKey: ["data-networks", dnPagination.page, dnPagination.pageSize],
    queryFn: () =>
      listDataNetworks(
        accessToken || "",
        dnPagination.page + 1,
        dnPagination.pageSize,
      ),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const dnRows: APIDataNetwork[] = dnPage?.items ?? [];
  const dnRowCount = dnPage?.total_count ?? 0;

  const [isCreateDNOpen, setCreateDNOpen] = useState(false);

  const handleOpenCreateDN = () => setCreateDNOpen(true);
  const handleCloseCreateDN = () => setCreateDNOpen(false);

  const outerTheme = useTheme();
  const gridTheme = useMemo(
    () =>
      createTheme(outerTheme, {
        palette: {
          DataGrid: { headerBg: outerTheme.palette.backgroundSubtle },
        },
      }),
    [outerTheme],
  );

  const dnDescription =
    "Manage the IP networks used by your subscribers. Data Network Names (DNNs) are used to identify different networks, and must be configured on the subscriber device to connect to the correct network.";

  const dnColumns: GridColDef<APIDataNetwork>[] = useMemo(() => {
    return [
      {
        field: "name",
        headerName: "Name (DNN)",
        flex: 1,
        minWidth: 200,
        renderCell: (params: GridRenderCellParams<APIDataNetwork>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Link
              to={`/networking/data-networks/${params.row.name}`}
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
                {params.row.name}
              </Typography>
            </Link>
          </Box>
        ),
      },
      { field: "ip_pool", headerName: "IP Pool", flex: 1, minWidth: 180 },
      {
        field: "sessions",
        headerName: "Sessions",
        width: 120,
        valueGetter: (_v, row) => row.status?.sessions ?? 0,
        renderCell: (params: GridRenderCellParams<APIDataNetwork>) => {
          const count = params.row.status?.sessions ?? 0;
          return (
            <Chip
              size="small"
              label={count}
              color={count > 0 ? "success" : "default"}
              variant="filled"
            />
          );
        },
      },
    ];
  }, [outerTheme]);

  // ====================== Interfaces ======================
  const {
    data: interfacesInfo,
    isLoading: interfacesLoading,
    refetch: refetchInterfaces,
  } = useQuery<InterfacesInfo>({
    queryKey: ["interfaces"],
    queryFn: () => getInterfaces(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
  });

  const interfacesDescription =
    "View the network interfaces used by Ella Core for control plane (N2), user plane (N3), external networks (N6), and the API endpoint. Interfaces are primarily configured in the Ella Core configuration file; this page reflects that configuration, with N3's external address as the only editable field.";

  // N3 edit modal state
  const [isEditN3Open, setEditN3Open] = useState(false);

  // ====================== Routes ======================
  const [rtPagination, setRtPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const {
    data: rtPage,
    isLoading: rtLoading,
    refetch: refetchRoutes,
  } = useQuery<ListRoutesResponse>({
    queryKey: ["routes", rtPagination.page, rtPagination.pageSize],
    queryFn: () =>
      listRoutes(
        accessToken || "",
        rtPagination.page + 1,
        rtPagination.pageSize,
      ),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const rtRows: APIRoute[] = rtPage?.items ?? [];
  const rtRowCount = rtPage?.total_count ?? 0;

  const [isCreateRouteOpen, setCreateRouteOpen] = useState(false);
  const [isDeleteRouteOpen, setDeleteRouteOpen] = useState(false);
  const [selectedRouteId, setSelectedRouteId] = useState<number | null>(null);

  const handleOpenCreateRoute = () => setCreateRouteOpen(true);
  const handleRequestDeleteRoute = (routeID: number) => {
    setSelectedRouteId(routeID);
    setDeleteRouteOpen(true);
  };

  const handleConfirmDeleteRoute = async () => {
    if (selectedRouteId == null || !accessToken) return;
    try {
      await deleteRoute(accessToken, selectedRouteId);
      setDeleteRouteOpen(false);
      showSnackbar(
        `Route "${selectedRouteId}" deleted successfully.`,
        "success",
      );
      refetchRoutes();
    } catch (error: unknown) {
      setDeleteRouteOpen(false);
      showSnackbar(
        `Failed to delete route "${selectedRouteId}": ${String(error)}`,
        "error",
      );
    } finally {
      setSelectedRouteId(null);
    }
  };

  const routesDescription =
    "Manage the routing table for subscriber traffic. Created routes are applied as kernel routes on the node running Ella Core.";

  const rtColumns: GridColDef<APIRoute>[] = useMemo(() => {
    return [
      { field: "id", headerName: "ID", width: 100 },
      {
        field: "destination",
        headerName: "Destination",
        flex: 1,
        minWidth: 180,
      },
      { field: "gateway", headerName: "Gateway", flex: 1, minWidth: 160 },
      { field: "interface", headerName: "Interface", flex: 1, minWidth: 140 },
      { field: "metric", headerName: "Metric", width: 110 },
      {
        field: "source",
        headerName: "Source",
        width: 110,
        renderCell: (params: GridRenderCellParams<APIRoute>) => {
          const source = params.row.source;
          return (
            <Chip
              size="small"
              label={source === "bgp" ? "BGP" : "Static"}
              color={source === "bgp" ? "info" : "default"}
              variant="outlined"
            />
          );
        },
      },
      ...(canEdit
        ? [
            {
              field: "actions",
              headerName: "Actions",
              type: "actions",
              width: 100,
              sortable: false,
              disableColumnMenu: true,
              getActions: (p: { row: APIRoute }) =>
                p.row.source === "bgp"
                  ? []
                  : [
                      <GridActionsCellItem
                        key="delete"
                        icon={<DeleteIcon color="primary" />}
                        label="Delete"
                        onClick={() => handleRequestDeleteRoute(p.row.id)}
                      />,
                    ],
            } as GridColDef<APIRoute>,
          ]
        : []),
    ];
  }, [canEdit]);

  // ====================== NAT ======================
  const {
    data: natInfo,
    isLoading: natLoading,
    refetch: refetchNAT,
  } = useQuery<NatInfo>({
    queryKey: ["nat"],
    queryFn: () => getNATInfo(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
  });

  const { mutate: setNATEnabled, isPending: natMutating } = useMutation<
    void,
    unknown,
    boolean
  >({
    mutationFn: (enabled: boolean) => updateNATInfo(accessToken || "", enabled),
    onSuccess: () => {
      showSnackbar("NAT updated successfully.", "success");
      refetchNAT();
    },
    onError: (error: unknown) => {
      showSnackbar(`Failed to update NAT: ${String(error)}`, "error");
    },
  });

  const natDescription =
    "Network Address Translation (NAT) simplifies networking as it lets subscribers use private IP addresses without requiring an external router. It uses Ella Core's N6 IP as the source for outbound traffic. Enabling NAT adds processing overhead and some niche protocols won't work (e.g., FTP active mode).";

  // ====================== Flow Accounting ======================
  const {
    data: flowAccountingInfo,
    isLoading: flowAccountingLoading,
    refetch: refetchFlowAccounting,
  } = useQuery<FlowAccountingInfo>({
    queryKey: ["flow-accounting"],
    queryFn: () => getFlowAccountingInfo(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
  });

  const {
    mutate: setFlowAccountingEnabled,
    isPending: flowAccountingMutating,
  } = useMutation<void, unknown, boolean>({
    mutationFn: (enabled: boolean) =>
      updateFlowAccountingInfo(accessToken || "", enabled),
    onSuccess: () => {
      showSnackbar("Flow accounting updated successfully.", "success");
      refetchFlowAccounting();
    },
    onError: (error: unknown) => {
      showSnackbar(
        `Failed to update flow accounting: ${String(error)}`,
        "error",
      );
    },
  });

  const flowAccountingDescription =
    "Flow accounting records per-flow network usage (source/destination IP and port, protocol, bytes, packets) for each subscriber session. Disabling flow accounting reduces processing overhead and stops collection of new flow data.";

  // ====================== BGP ======================
  const {
    data: bgpSettings,
    isLoading: bgpSettingsLoading,
    refetch: refetchBGPSettings,
  } = useQuery<BGPSettings>({
    queryKey: ["bgp-settings"],
    queryFn: () => getBGPSettings(accessToken || ""),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
  });

  const [isEditBGPSettingsOpen, setEditBGPSettingsOpen] = useState(false);

  const { mutate: setBGPEnabled, isPending: bgpToggling } = useMutation<
    void,
    unknown,
    boolean
  >({
    mutationFn: (enabled: boolean) =>
      updateBGPSettings(accessToken || "", {
        enabled,
        localAS: bgpSettings?.localAS ?? 64512,
        routerID: bgpSettings?.routerID ?? "",
        listenAddress: bgpSettings?.listenAddress ?? ":179",
      }),
    onSuccess: () => {
      showSnackbar("BGP settings updated successfully.", "success");
      refetchBGPSettings();
    },
    onError: (error: unknown) => {
      showSnackbar(`Failed to update BGP settings: ${String(error)}`, "error");
    },
  });

  // BGP Peers
  const [bgpPeersPagination, setBgpPeersPagination] =
    useState<GridPaginationModel>({ page: 0, pageSize: 25 });

  const {
    data: bgpPeersPage,
    isLoading: bgpPeersLoading,
    refetch: refetchBGPPeers,
  } = useQuery<ListBGPPeersResponse>({
    queryKey: [
      "bgp-peers",
      bgpPeersPagination.page,
      bgpPeersPagination.pageSize,
    ],
    queryFn: () =>
      listBGPPeers(
        accessToken || "",
        bgpPeersPagination.page + 1,
        bgpPeersPagination.pageSize,
      ),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const bgpPeerRows: APIBGPPeer[] = bgpPeersPage?.items ?? [];
  const bgpPeerRowCount = bgpPeersPage?.total_count ?? 0;

  const [isCreateBGPPeerOpen, setCreateBGPPeerOpen] = useState(false);
  const [isDeleteBGPPeerOpen, setDeleteBGPPeerOpen] = useState(false);
  const [selectedBGPPeerId, setSelectedBGPPeerId] = useState<number | null>(
    null,
  );

  const handleRequestDeleteBGPPeer = (id: number) => {
    setSelectedBGPPeerId(id);
    setDeleteBGPPeerOpen(true);
  };

  const handleConfirmDeleteBGPPeer = async () => {
    if (selectedBGPPeerId == null || !accessToken) return;
    try {
      await deleteBGPPeer(accessToken, selectedBGPPeerId);
      setDeleteBGPPeerOpen(false);
      showSnackbar("BGP peer deleted successfully.", "success");
      refetchBGPPeers();
    } catch (error: unknown) {
      setDeleteBGPPeerOpen(false);
      showSnackbar(`Failed to delete BGP peer: ${String(error)}`, "error");
    } finally {
      setSelectedBGPPeerId(null);
    }
  };

  const [isEditBGPPeerOpen, setEditBGPPeerOpen] = useState(false);
  const [editBGPPeer, setEditBGPPeer] = useState<APIBGPPeer | null>(null);

  const handleEditBGPPeer = (peer: APIBGPPeer) => {
    setEditBGPPeer(peer);
    setEditBGPPeerOpen(true);
  };

  const [isViewBGPPeerOpen, setViewBGPPeerOpen] = useState(false);
  const [viewBGPPeer, setViewBGPPeer] = useState<APIBGPPeer | null>(null);

  const handleViewBGPPeer = (peer: APIBGPPeer) => {
    setViewBGPPeer(peer);
    setViewBGPPeerOpen(true);
  };

  const getImportPolicyLabel = (prefixes: BGPImportPrefix[] | undefined) => {
    if (!prefixes || prefixes.length === 0) return "Deny All";
    if (
      prefixes.length === 1 &&
      prefixes[0].prefix === "0.0.0.0/0" &&
      prefixes[0].maxLength === 0
    )
      return "Default Route Only";
    if (
      prefixes.length === 1 &&
      prefixes[0].prefix === "0.0.0.0/0" &&
      prefixes[0].maxLength === 32
    )
      return "Accept All";
    return `${prefixes.length} ${prefixes.length === 1 ? "prefix" : "prefixes"}`;
  };

  const bgpPeerColumns: GridColDef<APIBGPPeer>[] = useMemo(() => {
    return [
      { field: "address", headerName: "Address", flex: 1, minWidth: 140 },
      { field: "remoteAS", headerName: "Remote AS", width: 120 },
      {
        field: "importPrefixes",
        headerName: "Import Policy",
        width: 160,
        sortable: false,
        renderCell: (params: GridRenderCellParams<APIBGPPeer>) =>
          getImportPolicyLabel(params.row.importPrefixes),
      },
      {
        field: "state",
        headerName: "Status",
        width: 200,
        renderCell: (params: GridRenderCellParams<APIBGPPeer>) => {
          const state = params.row.state;
          if (!state) return null;
          const label = state.charAt(0).toUpperCase() + state.slice(1);
          const text =
            state === "established" && params.row.uptime
              ? `${label} (${params.row.uptime})`
              : label;
          const color = state === "established" ? "success" : "default";
          return <Chip label={text} color={color} size="small" />;
        },
      },
      {
        field: "actions",
        headerName: "Actions",
        type: "actions",
        width: canEdit ? 140 : 60,
        sortable: false,
        disableColumnMenu: true,
        getActions: (p: { row: APIBGPPeer }) => [
          <GridActionsCellItem
            key="view"
            icon={<ViewIcon color="primary" />}
            label="View"
            onClick={() => handleViewBGPPeer(p.row)}
          />,
          ...(canEdit
            ? [
                <GridActionsCellItem
                  key="edit"
                  icon={<EditIcon color="primary" />}
                  label="Edit"
                  onClick={() => handleEditBGPPeer(p.row)}
                />,
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon color="primary" />}
                  label="Delete"
                  onClick={() => handleRequestDeleteBGPPeer(p.row.id)}
                />,
              ]
            : []),
        ],
      } as GridColDef<APIBGPPeer>,
    ];
  }, [canEdit]);

  // BGP Advertised Routes
  const { data: bgpAdvertisedData, isLoading: bgpAdvertisedLoading } =
    useQuery<BGPAdvertisedRoutesResponse>({
      queryKey: ["bgp-advertised-routes"],
      queryFn: () => getBGPAdvertisedRoutes(accessToken || ""),
      enabled: !!accessToken,
      refetchInterval: 5000,
      refetchIntervalInBackground: true,
      refetchOnWindowFocus: true,
    });

  const bgpAdvertisedRows: (BGPAdvertisedRoute & { id: string })[] = useMemo(
    () =>
      (bgpAdvertisedData?.routes ?? []).map((r, i) => ({
        ...r,
        id: `${r.prefix}-${i}`,
      })),
    [bgpAdvertisedData],
  );

  const bgpAdvertisedColumns: GridColDef<
    BGPAdvertisedRoute & { id: string }
  >[] = [
    {
      field: "subscriber",
      headerName: "Subscriber",
      flex: 1,
      minWidth: 180,
      renderCell: (params) => {
        const imsi = params.value as string;
        if (!imsi) return null;
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
              to={`/subscribers/${imsi}`}
              style={{ textDecoration: "none" }}
              onClick={(e: React.MouseEvent) => e.stopPropagation()}
            >
              <Typography
                variant="body2"
                sx={{
                  fontFamily: "monospace",
                  color: (t) => t.palette.link,
                  textDecoration: "underline",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                {imsi}
              </Typography>
            </Link>
          </Box>
        );
      },
    },
    { field: "prefix", headerName: "Prefix", flex: 1, minWidth: 160 },
    { field: "nextHop", headerName: "Next Hop", flex: 1, minWidth: 140 },
  ];

  // BGP Learned Routes
  const { data: bgpLearnedData, isLoading: bgpLearnedLoading } =
    useQuery<BGPLearnedRoutesResponse>({
      queryKey: ["bgp-learned-routes"],
      queryFn: () => getBGPLearnedRoutes(accessToken || ""),
      enabled: !!accessToken,
      refetchInterval: 5000,
      refetchIntervalInBackground: true,
      refetchOnWindowFocus: true,
    });

  const bgpLearnedRows: (BGPLearnedRoute & { id: string })[] = useMemo(
    () =>
      (bgpLearnedData?.routes ?? []).map((r, i) => ({
        ...r,
        id: `${r.prefix}-${i}`,
      })),
    [bgpLearnedData],
  );

  const bgpLearnedColumns: GridColDef<BGPLearnedRoute & { id: string }>[] = [
    { field: "prefix", headerName: "Prefix", flex: 1, minWidth: 160 },
    { field: "nextHop", headerName: "Next Hop", flex: 1, minWidth: 140 },
    { field: "peer", headerName: "Peer", flex: 1, minWidth: 140 },
  ];

  const isNATEnabled = !!natInfo?.enabled;

  const bgpDescription =
    "Border Gateway Protocol (BGP) allows Ella Core to advertise subscriber IP routes to upstream routers so that return traffic can reach connected UEs.";

  const handleTabChange = (_: React.SyntheticEvent, newValue: TabKey) => {
    setTab(newValue);
    setSearchParams({ tab: newValue }, { replace: true });
  };

  // ---------------- Render ----------------
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
      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: PAGE_PADDING_X }}>
        <Typography variant="h4" sx={{ mb: 1 }}>
          Networking
        </Typography>
        <Typography variant="body1" color="text.secondary" sx={{ mb: 2 }}>
          Configure networks and packet forwarding for Subscriber traffic.
        </Typography>

        <Tabs
          value={tab}
          onChange={handleTabChange}
          aria-label="Networking sections"
          sx={{ borderBottom: 1, borderColor: "divider" }}
        >
          <Tab value="data-networks" label="Data Networks" />
          <Tab value="interfaces" label="Interfaces" />
          <Tab value="routes" label="Routes" />
          <Tab value="nat" label="NAT" />
          <Tab value="bgp" label="BGP" />
          <Tab value="flow-accounting" label="Flow Accounting" />
        </Tabs>
      </Box>

      {/* ================= Data Networks Tab  ================= */}
      {tab === "data-networks" && (
        <Box
          sx={{
            width: "100%",
            mt: 2,
            maxWidth: MAX_WIDTH,
            px: PAGE_PADDING_X,
          }}
        >
          {dnLoading && dnRowCount === 0 ? (
            <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
              <CircularProgress />
            </Box>
          ) : dnRowCount === 0 ? (
            <EmptyState
              primaryText="No data network found."
              secondaryText="Create a data network to assign subscribers and control egress."
              extraContent={
                <Typography variant="body1" color="text.secondary">
                  {dnDescription}
                </Typography>
              }
              button={canEdit}
              buttonText="Create"
              onCreate={handleOpenCreateDN}
              readOnlyHint="Ask an administrator to create a data network."
            />
          ) : (
            <>
              <Box sx={{ mb: 3 }}>
                <Stack
                  direction={{ xs: "column", sm: "row" }}
                  spacing={2}
                  alignItems={{ xs: "stretch", sm: "center" }}
                  justifyContent="space-between"
                >
                  <Box>
                    <Typography variant="h5" sx={{ mb: 0.5 }}>
                      Data Networks ({dnRowCount})
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {dnDescription}
                    </Typography>
                    {canEdit && (
                      <Button
                        variant="contained"
                        color="success"
                        onClick={handleOpenCreateDN}
                        sx={{ maxWidth: 200, mt: 2 }}
                      >
                        Create
                      </Button>
                    )}
                  </Box>
                </Stack>
              </Box>
              <ThemeProvider theme={gridTheme}>
                <DataGrid<APIDataNetwork>
                  rows={dnRows}
                  columns={dnColumns}
                  getRowId={(row) => row.name}
                  paginationMode="server"
                  rowCount={dnRowCount}
                  paginationModel={dnPagination}
                  onPaginationModelChange={setDnPagination}
                  pageSizeOptions={[10, 25, 50, 100]}
                  disableColumnMenu
                  disableRowSelectionOnClick
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
            </>
          )}
        </Box>
      )}

      {/* ================= Interfaces Tab ================= */}
      {tab === "interfaces" && (
        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            px: PAGE_PADDING_X,
            mt: 2,
          }}
        >
          {interfacesLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
              <CircularProgress />
            </Box>
          ) : !interfacesInfo ? (
            <EmptyState
              primaryText="No interface information available."
              secondaryText="Ella Core could not retrieve interface information from the API."
              extraContent={
                <Typography variant="body1" color="text.secondary">
                  {interfacesDescription}
                </Typography>
              }
              button
              buttonText="Retry"
              onCreate={refetchInterfaces}
            />
          ) : (
            <>
              <Box sx={{ mb: 2 }}>
                <Typography variant="h5" sx={{ mb: 0.5 }}>
                  Network Interfaces
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {interfacesDescription}
                </Typography>
              </Box>

              <Box
                sx={{
                  display: "grid",
                  gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
                  gap: 2,
                  mt: 1,
                }}
              >
                {/* N2 */}
                <Box
                  sx={{
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 2,
                    p: 2,
                  }}
                >
                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    justifyContent="space-between"
                    sx={{ mb: 1 }}
                  >
                    <Typography variant="subtitle1">N2 (NGAP)</Typography>
                    <Chip label="Control Plane" size="small" />
                  </Stack>
                  <Typography variant="body2" color="text.secondary">
                    Address:{" "}
                    <strong>{interfacesInfo.n2?.address ?? "—"}</strong>
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    Port: <strong>{interfacesInfo.n2?.port ?? "—"}</strong>
                  </Typography>
                </Box>

                {/* N3 */}
                <Box
                  sx={{
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 2,
                    p: 2,
                  }}
                >
                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    justifyContent="space-between"
                    sx={{ mb: 1 }}
                  >
                    <Typography variant="subtitle1">N3 (GTP-U)</Typography>
                    <Stack direction="row" spacing={0.5} alignItems="center">
                      <Chip label="User Plane" size="small" />
                      {canEdit && (
                        <Tooltip title="Edit external address">
                          <IconButton
                            size="small"
                            onClick={() => setEditN3Open(true)}
                            color="primary"
                          >
                            <EditIcon fontSize="small" />
                          </IconButton>
                        </Tooltip>
                      )}
                    </Stack>
                  </Stack>
                  <Typography variant="body2" color="text.secondary">
                    Interface name:{" "}
                    <strong>{interfacesInfo.n3?.name ?? "—"}</strong>
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    Address:{" "}
                    <strong>{interfacesInfo.n3?.address ?? "—"}</strong>
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    External address:{" "}
                    <strong>
                      {interfacesInfo.n3?.external_address || "—"}
                    </strong>
                  </Typography>
                  {interfacesInfo.n3?.vlan && (
                    <Typography variant="body2" color="text.secondary">
                      VLAN:{" "}
                      <strong>
                        {interfacesInfo.n3.vlan.vlan_id ?? "—"}
                        {interfacesInfo.n3.vlan.master_interface
                          ? ` on ${interfacesInfo.n3.vlan.master_interface}`
                          : ""}
                      </strong>
                    </Typography>
                  )}
                </Box>

                {/* N6 */}
                <Box
                  sx={{
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 2,
                    p: 2,
                  }}
                >
                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    justifyContent="space-between"
                    sx={{ mb: 1 }}
                  >
                    <Typography variant="subtitle1">N6 (External)</Typography>
                    <Chip label="External Network" size="small" />
                  </Stack>
                  <Typography variant="body2" color="text.secondary">
                    Interface name:{" "}
                    <strong>{interfacesInfo.n6?.name ?? "—"}</strong>
                  </Typography>
                  {interfacesInfo.n6?.vlan && (
                    <Typography variant="body2" color="text.secondary">
                      VLAN:{" "}
                      <strong>
                        {interfacesInfo.n6.vlan.vlan_id ?? "—"}
                        {interfacesInfo.n6.vlan.master_interface
                          ? ` on ${interfacesInfo.n6.vlan.master_interface}`
                          : ""}
                      </strong>
                    </Typography>
                  )}
                </Box>

                {/* API */}
                <Box
                  sx={{
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 2,
                    p: 2,
                  }}
                >
                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    justifyContent="space-between"
                    sx={{ mb: 1 }}
                  >
                    <Typography variant="subtitle1">API</Typography>
                    <Chip label="Management" size="small" />
                  </Stack>
                  <Typography variant="body2" color="text.secondary">
                    Address:{" "}
                    <strong>{interfacesInfo.api?.address ?? "—"}</strong>
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    Port: <strong>{interfacesInfo.api?.port ?? "—"}</strong>
                  </Typography>
                </Box>
              </Box>
            </>
          )}
        </Box>
      )}

      {/* ================= Routes Tab  ================= */}
      {tab === "routes" && (
        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            px: PAGE_PADDING_X,
            mt: 2,
          }}
        >
          {rtLoading && rtRowCount === 0 ? (
            <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
              <CircularProgress />
            </Box>
          ) : rtRowCount === 0 ? (
            <EmptyState
              primaryText="No route found."
              secondaryText="Create a route so UEs can reach external networks."
              extraContent={
                <Typography variant="body1" color="text.secondary">
                  {routesDescription}
                </Typography>
              }
              button={canEdit}
              buttonText="Create"
              onCreate={handleOpenCreateRoute}
              readOnlyHint="Ask an administrator to create a route."
            />
          ) : (
            <>
              <Box sx={{ mb: 3 }}>
                <Stack
                  direction={{ xs: "column", sm: "row" }}
                  spacing={2}
                  alignItems={{ xs: "stretch", sm: "center" }}
                  justifyContent="space-between"
                >
                  <Box>
                    <Typography variant="h5" sx={{ mb: 0.5 }}>
                      Routes ({rtRowCount})
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {routesDescription}
                    </Typography>
                    {canEdit && (
                      <Button
                        variant="contained"
                        color="success"
                        onClick={handleOpenCreateRoute}
                        sx={{ maxWidth: 200, mt: 2 }}
                      >
                        Create
                      </Button>
                    )}
                  </Box>
                </Stack>
              </Box>

              <ThemeProvider theme={gridTheme}>
                <DataGrid<APIRoute>
                  rows={rtRows}
                  columns={rtColumns}
                  getRowId={(row) =>
                    row.source === "bgp"
                      ? `bgp-${row.destination}-${row.gateway}`
                      : row.id
                  }
                  paginationMode="server"
                  rowCount={rtRowCount}
                  paginationModel={rtPagination}
                  onPaginationModelChange={setRtPagination}
                  pageSizeOptions={[10, 25, 50, 100]}
                  disableColumnMenu
                  disableRowSelectionOnClick
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
            </>
          )}
        </Box>
      )}

      {/* ================= NAT Tab ================= */}
      {tab === "nat" && (
        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            px: PAGE_PADDING_X,
            mt: 2,
          }}
        >
          {natLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
              <CircularProgress />
            </Box>
          ) : (
            <>
              <Box sx={{ mb: 2 }}>
                <Typography variant="h5" sx={{ mb: 0.5 }}>
                  NAT
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {natDescription}
                </Typography>
              </Box>

              {bgpSettings?.enabled && !natInfo?.enabled && (
                <Alert severity="info" sx={{ mb: 2 }}>
                  Enabling NAT will stop advertising subscriber routes via BGP.
                </Alert>
              )}

              <Stack
                direction={{ xs: "column", sm: "row" }}
                spacing={2}
                alignItems="center"
              >
                <FormControlLabel
                  control={
                    <Switch
                      checked={!!natInfo?.enabled}
                      onChange={(_, checked) => setNATEnabled(checked)}
                      disabled={!canEdit || natMutating || natLoading}
                    />
                  }
                  label={natInfo?.enabled ? "NAT is ON" : "NAT is OFF"}
                />
              </Stack>
            </>
          )}
        </Box>
      )}

      {/* ================= BGP Tab ================= */}
      {tab === "bgp" && (
        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            px: PAGE_PADDING_X,
            mt: 2,
          }}
        >
          {bgpSettingsLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
              <CircularProgress />
            </Box>
          ) : (
            <>
              {/* --- Settings Card --- */}
              <Box sx={{ mb: 4 }}>
                <Typography variant="h5" sx={{ mb: 0.5 }}>
                  BGP Settings
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mb: 2 }}
                >
                  {bgpDescription}
                </Typography>

                <Stack
                  direction={{ xs: "column", sm: "row" }}
                  spacing={2}
                  alignItems={{ xs: "stretch", sm: "center" }}
                  justifyContent="space-between"
                  sx={{ mb: 2 }}
                >
                  <Stack direction="row" spacing={2} alignItems="center">
                    <FormControlLabel
                      control={
                        <Switch
                          checked={!!bgpSettings?.enabled}
                          onChange={(_, checked) => setBGPEnabled(checked)}
                          disabled={!canEdit || bgpToggling}
                        />
                      }
                      label={bgpSettings?.enabled ? "BGP is ON" : "BGP is OFF"}
                    />
                  </Stack>
                  {canEdit && (
                    <Button
                      variant="contained"
                      color="primary"
                      onClick={() => setEditBGPSettingsOpen(true)}
                      sx={{ maxWidth: 200 }}
                    >
                      Edit
                    </Button>
                  )}
                </Stack>

                <TableContainer
                  sx={{
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 1,
                  }}
                >
                  <Table>
                    <TableBody>
                      <TableRow>
                        <TableCell sx={{ fontWeight: 600, width: "35%" }}>
                          Local AS
                        </TableCell>
                        <TableCell>{bgpSettings?.localAS ?? 64512}</TableCell>
                      </TableRow>
                      <TableRow>
                        <TableCell sx={{ fontWeight: 600 }}>
                          Router ID
                        </TableCell>
                        <TableCell>{bgpSettings?.routerID || "N/A"}</TableCell>
                      </TableRow>
                      <TableRow>
                        <TableCell sx={{ fontWeight: 600 }}>
                          Listen Address
                        </TableCell>
                        <TableCell>
                          {bgpSettings?.listenAddress || ":179"}
                        </TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </TableContainer>
              </Box>

              {/* --- Peers Table --- */}
              <Box sx={{ mb: 4 }}>
                <Stack
                  direction={{ xs: "column", sm: "row" }}
                  spacing={2}
                  alignItems={{ xs: "stretch", sm: "center" }}
                  justifyContent="space-between"
                  sx={{ mb: 2 }}
                >
                  <Box>
                    <Typography variant="h5" sx={{ mb: 0.5 }}>
                      Peers ({bgpPeerRowCount})
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      Configured BGP neighbors. Changes are applied immediately.
                    </Typography>
                  </Box>
                  {canEdit && (
                    <Button
                      variant="contained"
                      color="success"
                      onClick={() => setCreateBGPPeerOpen(true)}
                      sx={{ maxWidth: 200 }}
                    >
                      Create
                    </Button>
                  )}
                </Stack>

                {bgpPeersLoading && bgpPeerRowCount === 0 ? (
                  <Box
                    sx={{ display: "flex", justifyContent: "center", mt: 4 }}
                  >
                    <CircularProgress />
                  </Box>
                ) : (
                  <>
                    <ThemeProvider theme={gridTheme}>
                      <DataGrid<APIBGPPeer>
                        rows={bgpPeerRows}
                        columns={bgpPeerColumns}
                        getRowId={(row) => row.id}
                        paginationMode="server"
                        rowCount={bgpPeerRowCount}
                        paginationModel={bgpPeersPagination}
                        onPaginationModelChange={setBgpPeersPagination}
                        pageSizeOptions={[10, 25, 50]}
                        disableColumnMenu
                        disableRowSelectionOnClick
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
                  </>
                )}
              </Box>

              {/* --- Advertised Routes Table --- */}
              <Box sx={{ mb: 4 }}>
                <Typography variant="h5" sx={{ mb: 0.5 }}>
                  Advertised Routes
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mb: 2 }}
                >
                  Routes currently announced to BGP peers. These are derived
                  from active PDU sessions and cannot be edited directly.
                </Typography>

                {isNATEnabled && (
                  <Alert severity="info" sx={{ mb: 2 }}>
                    Route advertisement is disabled while NAT is active. Disable
                    NAT to advertise subscriber routes to BGP peers.
                  </Alert>
                )}

                {bgpAdvertisedLoading ? (
                  <Box
                    sx={{ display: "flex", justifyContent: "center", mt: 4 }}
                  >
                    <CircularProgress />
                  </Box>
                ) : (
                  <ThemeProvider theme={gridTheme}>
                    <DataGrid
                      rows={bgpAdvertisedRows}
                      columns={bgpAdvertisedColumns}
                      disableColumnMenu
                      disableRowSelectionOnClick
                      pageSizeOptions={[10, 25, 50]}
                      slots={{
                        noRowsOverlay: () => (
                          <Stack
                            alignItems="center"
                            justifyContent="center"
                            sx={{ height: "100%", p: 2 }}
                          >
                            <Typography variant="body2" color="text.secondary">
                              No advertised routes
                            </Typography>
                          </Stack>
                        ),
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
                      }}
                    />
                  </ThemeProvider>
                )}
              </Box>

              {/* --- Learned Routes Table --- */}
              <Box sx={{ mb: 4 }}>
                <Typography variant="h5" sx={{ mb: 0.5 }}>
                  Learned Routes
                </Typography>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mb: 2 }}
                >
                  Routes received from BGP peers and installed in the kernel
                  routing table. These are controlled by each peer's import
                  policy.
                </Typography>

                {bgpLearnedLoading ? (
                  <Box
                    sx={{ display: "flex", justifyContent: "center", mt: 4 }}
                  >
                    <CircularProgress />
                  </Box>
                ) : (
                  <ThemeProvider theme={gridTheme}>
                    <DataGrid
                      rows={bgpLearnedRows}
                      columns={bgpLearnedColumns}
                      disableColumnMenu
                      disableRowSelectionOnClick
                      pageSizeOptions={[10, 25, 50]}
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
                )}
              </Box>
            </>
          )}
        </Box>
      )}

      {/* ================= Flow Accounting Tab ================= */}
      {tab === "flow-accounting" && (
        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            px: PAGE_PADDING_X,
            mt: 2,
          }}
        >
          {flowAccountingLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
              <CircularProgress />
            </Box>
          ) : (
            <>
              <Box sx={{ mb: 2 }}>
                <Typography variant="h5" sx={{ mb: 0.5 }}>
                  Flow Accounting
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {flowAccountingDescription}
                </Typography>
              </Box>

              <Stack
                direction={{ xs: "column", sm: "row" }}
                spacing={2}
                alignItems="center"
              >
                <FormControlLabel
                  control={
                    <Switch
                      checked={!!flowAccountingInfo?.enabled}
                      onChange={(_, checked) =>
                        setFlowAccountingEnabled(checked)
                      }
                      disabled={
                        !canEdit ||
                        flowAccountingMutating ||
                        flowAccountingLoading
                      }
                    />
                  }
                  label={
                    flowAccountingInfo?.enabled
                      ? "Flow accounting is ON"
                      : "Flow accounting is OFF"
                  }
                />
              </Stack>
            </>
          )}
        </Box>
      )}

      {/* ------------ Modals ------------ */}
      {isCreateDNOpen && (
        <CreateDataNetworkModal
          open
          onClose={handleCloseCreateDN}
          onSuccess={() => {
            refetchDataNetworks();
            showSnackbar("Data network created successfully.", "success");
          }}
        />
      )}

      {isEditN3Open && (
        <EditInterfaceN3Modal
          open
          onClose={() => setEditN3Open(false)}
          onSuccess={() => {
            showSnackbar(
              "N3 external address updated successfully.",
              "success",
            );
            refetchInterfaces();
          }}
          initialData={{
            externalAddress: interfacesInfo?.n3?.external_address ?? "",
          }}
        />
      )}

      {isCreateRouteOpen && (
        <CreateRouteModal
          open
          onClose={() => setCreateRouteOpen(false)}
          onSuccess={() => {
            refetchRoutes();
            showSnackbar("Route created successfully.", "success");
          }}
        />
      )}
      {isDeleteRouteOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteRouteOpen(false)}
          onConfirm={handleConfirmDeleteRoute}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the route "${selectedRouteId}"? This action cannot be undone.`}
        />
      )}

      {isCreateBGPPeerOpen && (
        <CreateBGPPeerModal
          open
          onClose={() => setCreateBGPPeerOpen(false)}
          onSuccess={() => {
            refetchBGPPeers();
            showSnackbar("BGP peer created successfully.", "success");
          }}
          rejectedPrefixes={bgpSettings?.rejectedPrefixes ?? []}
        />
      )}
      {isEditBGPPeerOpen && editBGPPeer && (
        <EditBGPPeerModal
          open
          onClose={() => setEditBGPPeerOpen(false)}
          onSuccess={() => {
            refetchBGPPeers();
            showSnackbar("BGP peer updated successfully.", "success");
          }}
          peer={editBGPPeer}
          rejectedPrefixes={bgpSettings?.rejectedPrefixes ?? []}
        />
      )}
      {isViewBGPPeerOpen && viewBGPPeer && (
        <ViewBGPPeerModal
          open
          onClose={() => setViewBGPPeerOpen(false)}
          peer={viewBGPPeer}
          rejectedPrefixes={bgpSettings?.rejectedPrefixes ?? []}
        />
      )}
      {isDeleteBGPPeerOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteBGPPeerOpen(false)}
          onConfirm={handleConfirmDeleteBGPPeer}
          title="Confirm Deletion"
          description={`Are you sure you want to delete this BGP peer? This action cannot be undone.`}
        />
      )}
      {isEditBGPSettingsOpen && bgpSettings && (
        <EditBGPSettingsModal
          open
          onClose={() => setEditBGPSettingsOpen(false)}
          onSuccess={() => {
            refetchBGPSettings();
            showSnackbar("BGP settings updated successfully.", "success");
          }}
          initialData={{
            enabled: bgpSettings.enabled,
            localAS: String(bgpSettings.localAS),
            routerID: bgpSettings.routerID,
            listenAddress: bgpSettings.listenAddress,
          }}
        />
      )}
    </Box>
  );
}
