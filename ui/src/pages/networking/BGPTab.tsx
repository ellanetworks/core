// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState, useMemo } from "react";
import {
  Alert,
  Box,
  Typography,
  Button,
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
import QueryState from "@/components/QueryState";
import { useNetworkingContext } from "./types";

const NoAdvertisedRoutesOverlay = () => (
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
);

const advertisedGridSlots = { noRowsOverlay: NoAdvertisedRoutesOverlay };

export default function BGPTab() {
  const { accessToken, canEdit, showSnackbar } = useNetworkingContext();

  const settingsQuery = useQuery<BGPSettings>({
    queryKey: ["bgp-settings"],
    queryFn: () => getBGPSettings(accessToken || ""),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    retry: false,
  });

  const bgpSettings = settingsQuery.data;
  const refetchSettings = () => void settingsQuery.refetch();

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

  const { data: natInfo } = useQuery<NatInfo>({
    queryKey: ["nat"],
    queryFn: () => getNATInfo(accessToken || ""),
    enabled: !!accessToken,
  });

  const isNATEnabled = !!natInfo?.enabled;

  const [peersPagination, setPeersPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const peersQuery = useQuery<ListBGPPeersResponse>({
    queryKey: ["bgp-peers", peersPagination.page, peersPagination.pageSize],
    queryFn: () =>
      listBGPPeers(
        accessToken || "",
        peersPagination.page + 1,
        peersPagination.pageSize,
      ),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    retry: false,
    placeholderData: (prev) => prev,
  });

  const refetchPeers = () => void peersQuery.refetch();
  const peerRowCount = peersQuery.data?.total_count;

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

  const advertisedQuery = useQuery<BGPAdvertisedRoutesResponse>({
    queryKey: ["bgp-advertised-routes"],
    queryFn: () => getBGPAdvertisedRoutes(accessToken || ""),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    retry: false,
  });

  const advertisedRows: (BGPAdvertisedRoute & { id: string })[] = useMemo(
    () =>
      (advertisedQuery.data?.routes ?? []).map((r, i) => ({
        ...r,
        id: `${r.prefix}-${i}`,
      })),
    [advertisedQuery.data],
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

  const learnedQuery = useQuery<BGPLearnedRoutesResponse>({
    queryKey: ["bgp-learned-routes"],
    queryFn: () => getBGPLearnedRoutes(accessToken || ""),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    retry: false,
  });

  const learnedRows: (BGPLearnedRoute & { id: string })[] = useMemo(
    () =>
      (learnedQuery.data?.routes ?? []).map((r, i) => ({
        ...r,
        id: `${r.prefix}-${i}`,
      })),
    [learnedQuery.data],
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
      <Box sx={{ mb: 4 }}>
        <Typography variant="h5" sx={{ mb: 0.5 }}>
          BGP Settings
        </Typography>
        <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
          {description}
        </Typography>

        <QueryState query={settingsQuery} resource="BGP settings">
          {(settings) => (
            <>
              <Stack
                direction={{ xs: "column", sm: "row" }}
                spacing={2}
                sx={{
                  alignItems: { xs: "stretch", sm: "center" },
                  justifyContent: "space-between",
                  mb: 2,
                }}
              >
                <Stack
                  direction="row"
                  spacing={2}
                  sx={{ alignItems: "center" }}
                >
                  <FormControlLabel
                    control={
                      <Switch
                        checked={!!settings.enabled}
                        onChange={(_, checked) => setBGPEnabled(checked)}
                        disabled={!canEdit || bgpToggling}
                      />
                    }
                    label={settings.enabled ? "BGP is ON" : "BGP is OFF"}
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
                      <TableCell>{settings.localAS ?? 64512}</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell sx={{ fontWeight: 600 }}>Router ID</TableCell>
                      <TableCell>{settings.routerID || "N/A"}</TableCell>
                    </TableRow>
                    <TableRow>
                      <TableCell sx={{ fontWeight: 600 }}>
                        Listen Address
                      </TableCell>
                      <TableCell>{settings.listenAddress || ":179"}</TableCell>
                    </TableRow>
                  </TableBody>
                </Table>
              </TableContainer>
            </>
          )}
        </QueryState>
      </Box>

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
              {peerRowCount === undefined ? "Peers" : `Peers (${peerRowCount})`}
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

        <QueryState query={peersQuery} resource="BGP peers">
          {(data) => (
            <DataGrid<APIBGPPeer>
              rows={data.items ?? []}
              columns={peerColumns}
              getRowId={(row) => row.id}
              paginationMode="server"
              rowCount={data.total_count ?? 0}
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
          )}
        </QueryState>
      </Box>

      <Box sx={{ mb: 4 }}>
        <Typography variant="h5" sx={{ mb: 0.5 }}>
          Advertised Routes
        </Typography>
        <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
          Routes currently announced to BGP peers. These are derived from active
          sessions and cannot be edited directly.
        </Typography>

        {isNATEnabled && (
          <Alert severity="info" sx={{ mb: 2 }}>
            Route advertisement is disabled while NAT is active. Disable NAT to
            advertise subscriber routes to BGP peers.
          </Alert>
        )}

        <QueryState query={advertisedQuery} resource="advertised routes">
          {() => (
            <DataGrid
              rows={advertisedRows}
              columns={advertisedColumns}
              disableColumnMenu
              disableRowSelectionOnClick
              pageSizeOptions={[10, 25, 50]}
              slots={advertisedGridSlots}
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
      </Box>

      <Box sx={{ mb: 4 }}>
        <Typography variant="h5" sx={{ mb: 0.5 }}>
          Learned Routes
        </Typography>
        <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
          Routes received from BGP peers and installed in the kernel routing
          table. These are controlled by each peer's import policy.
        </Typography>

        <QueryState query={learnedQuery} resource="learned routes">
          {() => (
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
          )}
        </QueryState>
      </Box>

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
