import React, { useState, useMemo } from "react";
import {
  Alert,
  Box,
  Typography,
  Button,
  CircularProgress,
  Chip,
  Stack,
  FormControlLabel,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
} from "@mui/material";
import { ThemeProvider } from "@mui/material/styles";
import {
  Delete as DeleteIcon,
  Edit as EditIcon,
  Visibility as ViewIcon,
} from "@mui/icons-material";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import {
  DataGrid,
  type GridColDef,
  GridActionsCellItem,
  type GridRenderCellParams,
  type GridPaginationModel,
} from "@mui/x-data-grid";
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
} from "@/queries/bgp";
import { getNATInfo, type NatInfo } from "@/queries/nat";
import CreateBGPPeerModal from "@/components/CreateBGPPeerModal";
import EditBGPPeerModal from "@/components/EditBGPPeerModal";
import ViewBGPPeerModal from "@/components/ViewBGPPeerModal";
import EditBGPSettingsModal from "@/components/EditBGPSettingsModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { useNetworkingContext } from "./types";

export default function BGPTab() {
  const { accessToken, canEdit, showSnackbar, gridTheme } =
    useNetworkingContext();

  // BGP Settings
  const {
    data: bgpSettings,
    isLoading: settingsLoading,
    refetch: refetchSettings,
  } = useQuery<BGPSettings>({
    queryKey: ["bgp-settings"],
    queryFn: () => getBGPSettings(accessToken || ""),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
  });

  const [isEditSettingsOpen, setEditSettingsOpen] = useState(false);

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
      refetchSettings();
    },
    onError: (error: unknown) => {
      showSnackbar(`Failed to update BGP settings: ${String(error)}`, "error");
    },
  });

  // NAT info (for the advertised routes warning)
  const { data: natInfo } = useQuery<NatInfo>({
    queryKey: ["nat"],
    queryFn: () => getNATInfo(accessToken || ""),
    enabled: !!accessToken,
  });

  const isNATEnabled = !!natInfo?.enabled;

  // BGP Peers
  const [peersPagination, setPeersPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const {
    data: peersPage,
    isLoading: peersLoading,
    refetch: refetchPeers,
  } = useQuery<ListBGPPeersResponse>({
    queryKey: ["bgp-peers", peersPagination.page, peersPagination.pageSize],
    queryFn: () =>
      listBGPPeers(
        accessToken || "",
        peersPagination.page + 1,
        peersPagination.pageSize,
      ),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const peerRows: APIBGPPeer[] = peersPage?.items ?? [];
  const peerRowCount = peersPage?.total_count ?? 0;

  const [isCreatePeerOpen, setCreatePeerOpen] = useState(false);
  const [isDeletePeerOpen, setDeletePeerOpen] = useState(false);
  const [selectedPeerId, setSelectedPeerId] = useState<number | null>(null);

  const handleRequestDeletePeer = (id: number) => {
    setSelectedPeerId(id);
    setDeletePeerOpen(true);
  };

  const handleConfirmDeletePeer = async () => {
    if (selectedPeerId == null || !accessToken) return;
    try {
      await deleteBGPPeer(accessToken, selectedPeerId);
      setDeletePeerOpen(false);
      showSnackbar("BGP peer deleted successfully.", "success");
      refetchPeers();
    } catch (error: unknown) {
      setDeletePeerOpen(false);
      showSnackbar(`Failed to delete BGP peer: ${String(error)}`, "error");
    } finally {
      setSelectedPeerId(null);
    }
  };

  const [isEditPeerOpen, setEditPeerOpen] = useState(false);
  const [editPeer, setEditPeer] = useState<APIBGPPeer | null>(null);

  const handleEditPeer = (peer: APIBGPPeer) => {
    setEditPeer(peer);
    setEditPeerOpen(true);
  };

  const [isViewPeerOpen, setViewPeerOpen] = useState(false);
  const [viewPeer, setViewPeer] = useState<APIBGPPeer | null>(null);

  const handleViewPeer = (peer: APIBGPPeer) => {
    setViewPeer(peer);
    setViewPeerOpen(true);
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

  const peerColumns: GridColDef<APIBGPPeer>[] = useMemo(() => {
    return [
      { field: "address", headerName: "Address", flex: 1, minWidth: 100 },
      { field: "remoteAS", headerName: "Remote AS", width: 100 },
      {
        field: "importPrefixes",
        headerName: "Import Policy",
        flex: 0.8,
        minWidth: 100,
        sortable: false,
        renderCell: (params: GridRenderCellParams<APIBGPPeer>) =>
          getImportPolicyLabel(params.row.importPrefixes),
      },
      {
        field: "state",
        headerName: "Status",
        flex: 1,
        minWidth: 120,
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
            onClick={() => handleViewPeer(p.row)}
          />,
          ...(canEdit
            ? [
                <GridActionsCellItem
                  key="edit"
                  icon={<EditIcon color="primary" />}
                  label="Edit"
                  onClick={() => handleEditPeer(p.row)}
                />,
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon color="primary" />}
                  label="Delete"
                  onClick={() => handleRequestDeletePeer(p.row.id)}
                />,
              ]
            : []),
        ],
      } as GridColDef<APIBGPPeer>,
    ];
  }, [canEdit]);

  // BGP Advertised Routes
  const { data: advertisedData, isLoading: advertisedLoading } =
    useQuery<BGPAdvertisedRoutesResponse>({
      queryKey: ["bgp-advertised-routes"],
      queryFn: () => getBGPAdvertisedRoutes(accessToken || ""),
      enabled: !!accessToken,
      refetchInterval: 5000,
      refetchIntervalInBackground: true,
      refetchOnWindowFocus: true,
    });

  const advertisedRows: (BGPAdvertisedRoute & { id: string })[] = useMemo(
    () =>
      (advertisedData?.routes ?? []).map((r, i) => ({
        ...r,
        id: `${r.prefix}-${i}`,
      })),
    [advertisedData],
  );

  const advertisedColumns: GridColDef<BGPAdvertisedRoute & { id: string }>[] = [
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
  const { data: learnedData, isLoading: learnedLoading } =
    useQuery<BGPLearnedRoutesResponse>({
      queryKey: ["bgp-learned-routes"],
      queryFn: () => getBGPLearnedRoutes(accessToken || ""),
      enabled: !!accessToken,
      refetchInterval: 5000,
      refetchIntervalInBackground: true,
      refetchOnWindowFocus: true,
    });

  const learnedRows: (BGPLearnedRoute & { id: string })[] = useMemo(
    () =>
      (learnedData?.routes ?? []).map((r, i) => ({
        ...r,
        id: `${r.prefix}-${i}`,
      })),
    [learnedData],
  );

  const learnedColumns: GridColDef<BGPLearnedRoute & { id: string }>[] = [
    { field: "prefix", headerName: "Prefix", flex: 1, minWidth: 160 },
    { field: "nextHop", headerName: "Next Hop", flex: 1, minWidth: 140 },
    { field: "peer", headerName: "Peer", flex: 1, minWidth: 140 },
  ];

  const description =
    "Border Gateway Protocol (BGP) allows Ella Core to advertise subscriber IP routes to upstream routers so that return traffic can reach connected UEs.";

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      {settingsLoading ? (
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
            <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
              {description}
            </Typography>

            <Stack
              direction={{ xs: "column", sm: "row" }}
              spacing={2}
              sx={{
                alignItems: { xs: "stretch", sm: "center" },
                justifyContent: "space-between",
                mb: 2,
              }}
            >
              <Stack direction="row" spacing={2} sx={{ alignItems: "center" }}>
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
                  onClick={() => setEditSettingsOpen(true)}
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
                    <TableCell sx={{ fontWeight: 600 }}>Router ID</TableCell>
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
              sx={{
                alignItems: { xs: "stretch", sm: "center" },
                justifyContent: "space-between",
                mb: 2,
              }}
            >
              <Box>
                <Typography variant="h5" sx={{ mb: 0.5 }}>
                  Peers ({peerRowCount})
                </Typography>
                <Typography variant="body2" color="textSecondary">
                  Configured BGP neighbors. Changes are applied immediately.
                </Typography>
              </Box>
              {canEdit && (
                <Button
                  variant="contained"
                  color="success"
                  onClick={() => setCreatePeerOpen(true)}
                  sx={{ maxWidth: 200 }}
                >
                  Create
                </Button>
              )}
            </Stack>

            {peersLoading && peerRowCount === 0 ? (
              <Box sx={{ display: "flex", justifyContent: "center", mt: 4 }}>
                <CircularProgress />
              </Box>
            ) : (
              <ThemeProvider theme={gridTheme}>
                <DataGrid<APIBGPPeer>
                  rows={peerRows}
                  columns={peerColumns}
                  getRowId={(row) => row.id}
                  paginationMode="server"
                  rowCount={peerRowCount}
                  paginationModel={peersPagination}
                  onPaginationModelChange={setPeersPagination}
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
            )}
          </Box>

          {/* --- Advertised Routes Table --- */}
          <Box sx={{ mb: 4 }}>
            <Typography variant="h5" sx={{ mb: 0.5 }}>
              Advertised Routes
            </Typography>
            <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
              Routes currently announced to BGP peers. These are derived from
              active PDU sessions and cannot be edited directly.
            </Typography>

            {isNATEnabled && (
              <Alert severity="info" sx={{ mb: 2 }}>
                Route advertisement is disabled while NAT is active. Disable NAT
                to advertise subscriber routes to BGP peers.
              </Alert>
            )}

            {advertisedLoading ? (
              <Box sx={{ display: "flex", justifyContent: "center", mt: 4 }}>
                <CircularProgress />
              </Box>
            ) : (
              <ThemeProvider theme={gridTheme}>
                <DataGrid
                  rows={advertisedRows}
                  columns={advertisedColumns}
                  disableColumnMenu
                  disableRowSelectionOnClick
                  pageSizeOptions={[10, 25, 50]}
                  slots={{
                    noRowsOverlay: () => (
                      <Stack
                        sx={{
                          alignItems: "center",
                          justifyContent: "center",
                          height: "100%",
                          p: 2,
                        }}
                      >
                        <Typography variant="body2" color="textSecondary">
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
            <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
              Routes received from BGP peers and installed in the kernel routing
              table. These are controlled by each peer's import policy.
            </Typography>

            {learnedLoading ? (
              <Box sx={{ display: "flex", justifyContent: "center", mt: 4 }}>
                <CircularProgress />
              </Box>
            ) : (
              <ThemeProvider theme={gridTheme}>
                <DataGrid
                  rows={learnedRows}
                  columns={learnedColumns}
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

      {/* --- Modals --- */}
      {isCreatePeerOpen && (
        <CreateBGPPeerModal
          open
          onClose={() => setCreatePeerOpen(false)}
          onSuccess={() => {
            refetchPeers();
            showSnackbar("BGP peer created successfully.", "success");
          }}
          rejectedPrefixes={bgpSettings?.rejectedPrefixes ?? []}
        />
      )}
      {isEditPeerOpen && editPeer && (
        <EditBGPPeerModal
          open
          onClose={() => setEditPeerOpen(false)}
          onSuccess={() => {
            refetchPeers();
            showSnackbar("BGP peer updated successfully.", "success");
          }}
          peer={editPeer}
          rejectedPrefixes={bgpSettings?.rejectedPrefixes ?? []}
        />
      )}
      {isViewPeerOpen && viewPeer && (
        <ViewBGPPeerModal
          open
          onClose={() => setViewPeerOpen(false)}
          peer={viewPeer}
          rejectedPrefixes={bgpSettings?.rejectedPrefixes ?? []}
        />
      )}
      {isDeletePeerOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeletePeerOpen(false)}
          onConfirm={handleConfirmDeletePeer}
          title="Confirm Deletion"
          description="Are you sure you want to delete this BGP peer? This action cannot be undone."
        />
      )}
      {isEditSettingsOpen && bgpSettings && (
        <EditBGPSettingsModal
          open
          onClose={() => setEditSettingsOpen(false)}
          onSuccess={() => {
            refetchSettings();
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
