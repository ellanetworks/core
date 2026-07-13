// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  IconButton,
  LinearProgress,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import {
  Add as AddIcon,
  Edit as EditIcon,
  Delete as DeleteIcon,
} from "@mui/icons-material";
import { Link as RouterLink, useNavigate, useParams } from "react-router-dom";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  DataGrid,
  type GridColDef,
  type GridPaginationModel,
  type GridRenderCellParams,
} from "@mui/x-data-grid";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  getDataNetwork,
  deleteDataNetwork,
  listIPv4Allocations,
  listIPv6Allocations,
  deleteStaticIp,
  listFramedRoutes,
  deleteFramedRoute,
  type APIDataNetwork,
  type APIIPAllocation,
} from "@/queries/data_networks";
import { getNATInfo, type NatInfo } from "@/queries/nat";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import EditDataNetworkModal from "@/components/EditDataNetworkModal";
import CreateStaticIpModal from "@/components/CreateStaticIpModal";
import CreateFramedRouteModal from "@/components/CreateFramedRouteModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const tableContainerSx = {
  border: 1,
  borderColor: "divider",
  borderRadius: 1,
} as const;

const labelCellSx = { fontWeight: 600, width: "35%" } as const;
const valueCellSx = { width: "65%" } as const;

const GRID_HEIGHT = 421;

const DataNetworkDetail: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();
  const { role, accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const theme = useTheme();
  const canEdit = role === "Admin" || role === "Network Manager";

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: theme.palette.backgroundSubtle } },
      }),
    [theme],
  );

  const queryClient = useQueryClient();
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isDeleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [staticModal, setStaticModal] = useState<
    | { mode: "create" }
    | { mode: "edit"; imsi: string; ipVersion: string; address: string }
    | null
  >(null);
  const [deleteStatic, setDeleteStatic] = useState<{
    imsi: string;
    ipVersion: string;
    address: string;
  } | null>(null);
  const [framedModal, setFramedModal] = useState<
    | { mode: "create" }
    | { mode: "edit"; imsi: string; ipv4: string[]; ipv6: string[] }
    | null
  >(null);
  const [deleteFramed, setDeleteFramed] = useState<{ imsi: string } | null>(
    null,
  );

  useEffect(() => {
    if (authReady && !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const {
    data: dataNetwork,
    isLoading,
    error,
    refetch,
  } = useQuery<APIDataNetwork>({
    queryKey: ["data-network", name],
    queryFn: () => getDataNetwork(accessToken!, name!),
    enabled: authReady && !!accessToken && !!name,
    refetchInterval: 5000,
  });

  const [allocPaginationModel, setAllocPaginationModel] =
    useState<GridPaginationModel>({ page: 0, pageSize: 10 });

  const allocPage = allocPaginationModel.page + 1;
  const allocPerPage = allocPaginationModel.pageSize;

  const { data: allocationsData } = useQuery({
    queryKey: ["ipv4-allocations", name, allocPage, allocPerPage],
    queryFn: () =>
      listIPv4Allocations(accessToken!, name!, allocPage, allocPerPage),
    enabled: authReady && !!accessToken && !!name && !!dataNetwork?.ipv4_pool,
    refetchInterval: 5000,
  });

  const [ipv6AllocPaginationModel, setIpv6AllocPaginationModel] =
    useState<GridPaginationModel>({ page: 0, pageSize: 10 });

  const ipv6AllocPage = ipv6AllocPaginationModel.page + 1;
  const ipv6AllocPerPage = ipv6AllocPaginationModel.pageSize;

  const { data: ipv6AllocationsData } = useQuery({
    queryKey: ["ipv6-allocations", name, ipv6AllocPage, ipv6AllocPerPage],
    queryFn: () =>
      listIPv6Allocations(accessToken!, name!, ipv6AllocPage, ipv6AllocPerPage),
    enabled: authReady && !!accessToken && !!name && !!dataNetwork?.ipv6_pool,
    refetchInterval: 5000,
  });

  const handleDeleteConfirm = async () => {
    if (!name || !accessToken) return;

    try {
      await deleteDataNetwork(accessToken, name);
      setDeleteConfirmOpen(false);
      showSnackbar(`Data network "${name}" deleted successfully.`, "success");
      navigate("/networking/data-networks");
    } catch (err) {
      setDeleteConfirmOpen(false);
      showSnackbar(
        `Failed to delete data network: ${err instanceof Error ? err.message : "Unknown error"}`,
        "error",
      );
    }
  };

  const { data: framedRoutesData } = useQuery({
    queryKey: ["framed-routes", name],
    queryFn: () => listFramedRoutes(accessToken!, name!),
    enabled: authReady && !!accessToken && !!name,
  });
  const framedRoutes = framedRoutesData ?? [];

  const { data: natInfo } = useQuery<NatInfo>({
    queryKey: ["nat"],
    queryFn: () => getNATInfo(accessToken || ""),
    enabled: authReady && !!accessToken,
  });
  const isNATEnabled = !!natInfo?.enabled;

  const invalidateFramed = () => {
    queryClient.invalidateQueries({ queryKey: ["framed-routes", name] });
  };

  const handleFramedDeleteConfirm = async () => {
    if (!deleteFramed || !accessToken || !name) return;

    try {
      await deleteFramedRoute(accessToken, name, deleteFramed.imsi);
      setDeleteFramed(null);
      invalidateFramed();
      showSnackbar("Framed routes deleted successfully.", "success");
    } catch (err) {
      setDeleteFramed(null);
      showSnackbar(
        `Failed to delete framed routes: ${err instanceof Error ? err.message : "Unknown error"}`,
        "error",
      );
    }
  };

  const renderPrefixChips = (prefixes?: string[]) =>
    prefixes && prefixes.length > 0 ? (
      prefixes.map((p) => (
        <Chip
          key={p}
          label={p}
          size="small"
          sx={{ mr: 0.5, mb: 0.5, fontFamily: "monospace" }}
        />
      ))
    ) : (
      <Typography variant="body2" color="textSecondary" component="span">
        —
      </Typography>
    );

  const invalidateStaticIps = () => {
    queryClient.invalidateQueries({ queryKey: ["ipv4-allocations", name] });
    queryClient.invalidateQueries({ queryKey: ["ipv6-allocations", name] });
    refetch();
  };

  const handleStaticDeleteConfirm = async () => {
    if (!deleteStatic || !accessToken || !name) return;

    try {
      await deleteStaticIp(
        accessToken,
        name,
        deleteStatic.imsi,
        deleteStatic.ipVersion,
      );
      setDeleteStatic(null);
      invalidateStaticIps();
      showSnackbar("Static IP deleted successfully.", "success");
    } catch (err) {
      setDeleteStatic(null);
      showSnackbar(
        `Failed to delete static IP: ${err instanceof Error ? err.message : "Unknown error"}`,
        "error",
      );
    }
  };

  const staticActionsColumn = useCallback(
    (ipVersion: string): GridColDef<APIIPAllocation> => ({
      field: "actions",
      headerName: "Actions",
      width: 100,
      sortable: false,
      renderCell: (params: GridRenderCellParams<APIIPAllocation>) => {
        if (params.row.type !== "static") return null;

        const active = params.row.session_id != null;

        return (
          <Box sx={{ display: "flex", alignItems: "center", height: "100%" }}>
            <IconButton
              size="small"
              disabled={active}
              aria-label="Edit static IP"
              onClick={(e: React.MouseEvent) => {
                e.stopPropagation();
                setStaticModal({
                  mode: "edit",
                  imsi: params.row.imsi,
                  ipVersion,
                  address: params.row.address,
                });
              }}
            >
              <EditIcon fontSize="small" />
            </IconButton>
            <IconButton
              size="small"
              color="error"
              disabled={active}
              aria-label="Delete static IP"
              onClick={(e: React.MouseEvent) => {
                e.stopPropagation();
                setDeleteStatic({
                  imsi: params.row.imsi,
                  ipVersion,
                  address: params.row.address,
                });
              }}
            >
              <DeleteIcon fontSize="small" />
            </IconButton>
          </Box>
        );
      },
    }),
    [],
  );

  const allocationColumns: GridColDef<APIIPAllocation>[] = useMemo(() => {
    const cols: GridColDef<APIIPAllocation>[] = [
      {
        field: "address",
        headerName: "Address",
        flex: 1,
        minWidth: 140,
        renderCell: (params: GridRenderCellParams<APIIPAllocation>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
              {params.row.address}
            </Typography>
          </Box>
        ),
      },
      {
        field: "imsi",
        headerName: "Subscriber",
        flex: 1,
        minWidth: 140,
        renderCell: (params: GridRenderCellParams<APIIPAllocation>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <RouterLink
              to={`/subscribers/${params.row.imsi}`}
              style={{ textDecoration: "none" }}
              onClick={(e: React.MouseEvent) => e.stopPropagation()}
            >
              <Typography
                variant="body2"
                sx={{
                  fontFamily: "monospace",
                  color: theme.palette.link,
                  textDecoration: "underline",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                {params.row.imsi}
              </Typography>
            </RouterLink>
          </Box>
        ),
      },
      {
        field: "type",
        headerName: "Type",
        width: 120,
        renderCell: (params: GridRenderCellParams<APIIPAllocation>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Chip
              size="small"
              label={params.row.type}
              color={params.row.type === "static" ? "primary" : "default"}
              variant="outlined"
            />
          </Box>
        ),
      },
      {
        field: "session_id",
        headerName: "Session ID",
        flex: 0.5,
        minWidth: 100,
        renderCell: (params: GridRenderCellParams<APIIPAllocation>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Typography
              variant="body2"
              sx={
                params.row.session_id != null ? {} : { color: "text.secondary" }
              }
            >
              {params.row.session_id != null ? params.row.session_id : "—"}
            </Typography>
          </Box>
        ),
      },
    ];

    if (canEdit) {
      cols.push(staticActionsColumn("ipv4"));
    }

    return cols;
  }, [theme, canEdit, staticActionsColumn]);

  const ipv6AllocationColumns: GridColDef<APIIPAllocation>[] = useMemo(() => {
    const cols: GridColDef<APIIPAllocation>[] = [
      {
        field: "address",
        headerName: "Prefix",
        flex: 1,
        minWidth: 180,
        renderCell: (params: GridRenderCellParams<APIIPAllocation>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
              {params.row.address.includes(":")
                ? `${params.row.address}/64`
                : params.row.address}
            </Typography>
          </Box>
        ),
      },
      {
        field: "imsi",
        headerName: "Subscriber",
        flex: 1,
        minWidth: 140,
        renderCell: (params: GridRenderCellParams<APIIPAllocation>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <RouterLink
              to={`/subscribers/${params.row.imsi}`}
              style={{ textDecoration: "none" }}
              onClick={(e: React.MouseEvent) => e.stopPropagation()}
            >
              <Typography
                variant="body2"
                sx={{
                  fontFamily: "monospace",
                  color: theme.palette.link,
                  textDecoration: "underline",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                {params.row.imsi}
              </Typography>
            </RouterLink>
          </Box>
        ),
      },
      {
        field: "type",
        headerName: "Type",
        width: 120,
        renderCell: (params: GridRenderCellParams<APIIPAllocation>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Chip
              size="small"
              label={params.row.type}
              color={params.row.type === "static" ? "primary" : "default"}
              variant="outlined"
            />
          </Box>
        ),
      },
      {
        field: "session_id",
        headerName: "Session ID",
        flex: 0.5,
        minWidth: 100,
        renderCell: (params: GridRenderCellParams<APIIPAllocation>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Typography
              variant="body2"
              sx={
                params.row.session_id != null ? {} : { color: "text.secondary" }
              }
            >
              {params.row.session_id != null ? params.row.session_id : "—"}
            </Typography>
          </Box>
        ),
      },
    ];

    if (canEdit) {
      cols.push(staticActionsColumn("ipv6"));
    }

    return cols;
  }, [theme, canEdit, staticActionsColumn]);

  if (!authReady || isLoading) {
    return (
      <Box
        sx={{
          pt: 6,
          pb: 4,
          maxWidth: MAX_WIDTH,
          mx: "auto",
          px: PAGE_PADDING_X,
        }}
      >
        <Skeleton variant="text" width={320} height={48} sx={{ mb: 3 }} />
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            gap: 3,
          }}
        >
          <Skeleton variant="rounded" height={220} />
          <Skeleton variant="rounded" height={220} />
        </Box>
        <Skeleton variant="rounded" height={300} sx={{ mt: 3 }} />
      </Box>
    );
  }

  if (error) {
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          mt: 6,
          gap: 2,
        }}
      >
        <Typography color="error">
          {error instanceof Error
            ? error.message
            : "Failed to load data network."}
        </Typography>
        <Button
          variant="outlined"
          component={RouterLink}
          to="/networking/data-networks"
        >
          Back to Networking
        </Button>
      </Box>
    );
  }

  if (!dataNetwork) return null;

  const allocationRows = allocationsData?.items ?? [];
  const allocationRowCount = allocationsData?.total_count ?? 0;
  const ipv6AllocationRows = ipv6AllocationsData?.items ?? [];
  const ipv6AllocationRowCount = ipv6AllocationsData?.total_count ?? 0;
  const hasIpv4Pool = !!dataNetwork.ipv4_pool;
  const hasIpv6Pool = !!dataNetwork.ipv6_pool;
  const ipAlloc = dataNetwork.ip_allocation;
  const poolSize = ipAlloc?.pool_size ?? 0;
  const allocated = ipAlloc?.allocated ?? 0;
  const utilPercent = poolSize > 0 ? (allocated / poolSize) * 100 : 0;
  const ipv6Alloc = dataNetwork.ipv6_allocation;
  const ipv6PoolSize = ipv6Alloc?.pool_size ?? 0;
  const ipv6Allocated = ipv6Alloc?.allocated ?? 0;
  const ipv6UtilPercent =
    ipv6PoolSize > 0 ? (ipv6Allocated / ipv6PoolSize) * 100 : 0;

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      {/* Header / Breadcrumb */}
      <Box
        sx={{
          display: "flex",
          flexDirection: { xs: "column", sm: "row" },
          alignItems: { xs: "flex-start", sm: "center" },
          gap: 2,
          mb: 3,
        }}
      >
        <Box sx={{ flex: 1 }}>
          <Typography
            variant="h4"
            sx={{ display: "flex", alignItems: "baseline", gap: 0 }}
          >
            <Typography
              component={RouterLink}
              to="/networking/data-networks"
              variant="h4"
              sx={{
                color: "text.secondary",
                textDecoration: "none",
                "&:hover": { textDecoration: "underline" },
              }}
            >
              Data Networks
            </Typography>
            <Typography
              component="span"
              variant="h4"
              sx={{ color: "text.secondary", mx: 1 }}
            >
              /
            </Typography>
            <Typography component="span" variant="h4">
              {dataNetwork.name}
            </Typography>
          </Typography>
        </Box>
        {canEdit && (
          <Box sx={{ display: "flex", gap: 1 }}>
            <Button
              variant="outlined"
              color="error"
              onClick={() => setDeleteConfirmOpen(true)}
            >
              Delete
            </Button>
          </Box>
        )}
      </Box>

      {/* Two-column layout: Configuration + Status */}
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
          gap: 3,
          alignItems: "stretch",
        }}
      >
        {/* Configuration Card */}
        <Card
          variant="outlined"
          sx={{ height: "100%", display: "flex", flexDirection: "column" }}
        >
          <CardContent
            sx={{ flex: 1, display: "flex", flexDirection: "column" }}
          >
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                mb: 1.5,
              }}
            >
              <Typography variant="h6">Configuration</Typography>
              {canEdit && (
                <IconButton
                  size="small"
                  color="primary"
                  onClick={() => setEditModalOpen(true)}
                  aria-label="Edit configuration"
                >
                  <EditIcon fontSize="small" />
                </IconButton>
              )}
            </Box>
            <Table
              size="small"
              sx={{
                "& tr:last-child td": { borderBottom: "none" },
              }}
            >
              <TableBody>
                <TableRow>
                  <TableCell sx={labelCellSx}>IPv4 Pool</TableCell>
                  <TableCell sx={valueCellSx}>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace" }}
                    >
                      {dataNetwork.ipv4_pool}
                    </Typography>
                  </TableCell>
                </TableRow>
                {dataNetwork.ipv6_pool && (
                  <TableRow>
                    <TableCell sx={labelCellSx}>IPv6 Pool</TableCell>
                    <TableCell sx={valueCellSx}>
                      <Typography
                        variant="body2"
                        sx={{ fontFamily: "monospace" }}
                      >
                        {dataNetwork.ipv6_pool}
                      </Typography>
                    </TableCell>
                  </TableRow>
                )}
                <TableRow>
                  <TableCell sx={labelCellSx}>DNS</TableCell>
                  <TableCell sx={valueCellSx}>
                    <Typography
                      variant="body2"
                      sx={{ fontFamily: "monospace" }}
                    >
                      {dataNetwork.dns || "—"}
                    </Typography>
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell sx={labelCellSx}>MTU</TableCell>
                  <TableCell sx={valueCellSx}>
                    {dataNetwork.mtu || "—"}
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        {/* Status Card */}
        <Card
          variant="outlined"
          sx={{ height: "100%", display: "flex", flexDirection: "column" }}
        >
          <CardContent
            sx={{ flex: 1, display: "flex", flexDirection: "column" }}
          >
            <Typography variant="h6" sx={{ mb: 1.5 }}>
              Status
            </Typography>
            <Table
              size="small"
              sx={{
                "& tr:last-child td": { borderBottom: "none" },
              }}
            >
              <TableBody>
                <TableRow>
                  <TableCell sx={labelCellSx}>Active Sessions</TableCell>
                  <TableCell sx={valueCellSx}>
                    <Chip
                      size="small"
                      label={dataNetwork.status?.sessions ?? 0}
                      variant="outlined"
                    />
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell sx={labelCellSx}>IPv4 Pool Utilization</TableCell>
                  <TableCell sx={valueCellSx}>
                    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                      <LinearProgress
                        variant="determinate"
                        value={Math.min(utilPercent, 100)}
                        sx={{ flex: 1, height: 8, borderRadius: 4 }}
                      />
                      <Typography variant="body2" sx={{ whiteSpace: "nowrap" }}>
                        {allocated.toLocaleString()} /{" "}
                        {poolSize.toLocaleString()}
                      </Typography>
                    </Box>
                  </TableCell>
                </TableRow>
                {dataNetwork.ipv6_pool && (
                  <TableRow>
                    <TableCell sx={labelCellSx}>
                      IPv6 Pool Utilization
                    </TableCell>
                    <TableCell sx={valueCellSx}>
                      <Box
                        sx={{ display: "flex", alignItems: "center", gap: 1 }}
                      >
                        <LinearProgress
                          variant="determinate"
                          value={Math.min(ipv6UtilPercent, 100)}
                          sx={{ flex: 1, height: 8, borderRadius: 4 }}
                        />
                        <Typography
                          variant="body2"
                          sx={{ whiteSpace: "nowrap" }}
                        >
                          {ipv6Allocated.toLocaleString()} /{" "}
                          {ipv6PoolSize.toLocaleString()}
                        </Typography>
                      </Box>
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      </Box>

      {/* IP Allocations */}
      {(hasIpv4Pool || hasIpv6Pool) && (
        <Box sx={{ mt: 3 }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              mb: 1.5,
            }}
          >
            <Typography variant="h6">IP Allocations</Typography>
            {canEdit && (
              <Button
                variant="outlined"
                size="small"
                startIcon={<AddIcon />}
                onClick={() => setStaticModal({ mode: "create" })}
              >
                Add static IP
              </Button>
            )}
          </Box>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: {
                xs: "1fr",
                md: hasIpv4Pool && hasIpv6Pool ? "1fr 1fr" : "1fr",
              },
              gap: 2,
              alignItems: "start",
            }}
          >
            {hasIpv4Pool && (
              <Box>
                <Typography variant="subtitle1" sx={{ mb: 1 }}>
                  IPv4 Allocations ({allocationRowCount.toLocaleString()})
                </Typography>
                {allocationRowCount === 0 ? (
                  <TableContainer sx={tableContainerSx}>
                    <Box sx={{ p: 3, textAlign: "center" }}>
                      <Typography variant="body2" color="textSecondary">
                        No IPv4 addresses are currently allocated in this pool.
                      </Typography>
                    </Box>
                  </TableContainer>
                ) : (
                  <ThemeProvider theme={gridTheme}>
                    <DataGrid<APIIPAllocation>
                      rows={allocationRows}
                      columns={allocationColumns}
                      getRowId={(row) => row.address}
                      paginationMode="server"
                      rowCount={allocationRowCount}
                      paginationModel={allocPaginationModel}
                      onPaginationModelChange={setAllocPaginationModel}
                      pageSizeOptions={[10, 25]}
                      disableColumnMenu
                      disableRowSelectionOnClick
                      density="compact"
                      sx={{
                        height: GRID_HEIGHT,
                        border: 1,
                        borderColor: "divider",
                        "& .MuiDataGrid-cell": {
                          borderBottom: "1px solid",
                          borderColor: "divider",
                        },
                      }}
                    />
                  </ThemeProvider>
                )}
              </Box>
            )}
            {hasIpv6Pool && (
              <Box>
                <Typography variant="subtitle1" sx={{ mb: 1 }}>
                  IPv6 Allocations ({ipv6AllocationRowCount.toLocaleString()})
                </Typography>
                {ipv6AllocationRowCount === 0 ? (
                  <TableContainer sx={tableContainerSx}>
                    <Box sx={{ p: 3, textAlign: "center" }}>
                      <Typography variant="body2" color="textSecondary">
                        No IPv6 prefixes are currently allocated in this pool.
                      </Typography>
                    </Box>
                  </TableContainer>
                ) : (
                  <ThemeProvider theme={gridTheme}>
                    <DataGrid<APIIPAllocation>
                      rows={ipv6AllocationRows}
                      columns={ipv6AllocationColumns}
                      getRowId={(row) => row.address}
                      paginationMode="server"
                      rowCount={ipv6AllocationRowCount}
                      paginationModel={ipv6AllocPaginationModel}
                      onPaginationModelChange={setIpv6AllocPaginationModel}
                      pageSizeOptions={[10, 25]}
                      disableColumnMenu
                      disableRowSelectionOnClick
                      density="compact"
                      sx={{
                        height: GRID_HEIGHT,
                        border: 1,
                        borderColor: "divider",
                        "& .MuiDataGrid-cell": {
                          borderBottom: "1px solid",
                          borderColor: "divider",
                        },
                      }}
                    />
                  </ThemeProvider>
                )}
              </Box>
            )}
          </Box>
        </Box>
      )}

      {/* Framed Routes */}
      <Box sx={{ mt: 4 }}>
        <Box
          sx={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            mb: 1.5,
          }}
        >
          <Typography variant="h6">Framed Routes</Typography>
          {canEdit && (
            <Tooltip
              title={
                isNATEnabled ? "Framed routes require NAT to be disabled." : ""
              }
            >
              <span>
                <Button
                  variant="outlined"
                  size="small"
                  startIcon={<AddIcon />}
                  onClick={() => setFramedModal({ mode: "create" })}
                  disabled={isNATEnabled}
                >
                  Add framed route
                </Button>
              </span>
            </Tooltip>
          )}
        </Box>
        {isNATEnabled && (
          <Alert severity="info" sx={{ mb: 2 }}>
            Framed routes require NAT to be disabled. Disable NAT to add and
            forward framed routes.
          </Alert>
        )}
        {framedRoutes.length === 0 ? (
          <TableContainer sx={tableContainerSx}>
            <Box sx={{ p: 3, textAlign: "center" }}>
              <Typography variant="body2" color="textSecondary">
                No framed routes are configured for this data network.
              </Typography>
            </Box>
          </TableContainer>
        ) : (
          <TableContainer sx={tableContainerSx}>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Subscriber (IMSI)</TableCell>
                  <TableCell>IPv4 prefixes</TableCell>
                  <TableCell>IPv6 prefixes</TableCell>
                  {canEdit && <TableCell align="right">Actions</TableCell>}
                </TableRow>
              </TableHead>
              <TableBody>
                {framedRoutes.map((fr) => (
                  <TableRow key={fr.imsi}>
                    <TableCell sx={{ fontFamily: "monospace" }}>
                      {fr.imsi}
                    </TableCell>
                    <TableCell>{renderPrefixChips(fr.ipv4)}</TableCell>
                    <TableCell>{renderPrefixChips(fr.ipv6)}</TableCell>
                    {canEdit && (
                      <TableCell align="right">
                        <IconButton
                          size="small"
                          aria-label="Edit framed routes"
                          onClick={() =>
                            setFramedModal({
                              mode: "edit",
                              imsi: fr.imsi,
                              ipv4: fr.ipv4 ?? [],
                              ipv6: fr.ipv6 ?? [],
                            })
                          }
                        >
                          <EditIcon fontSize="small" />
                        </IconButton>
                        <IconButton
                          size="small"
                          aria-label="Delete framed routes"
                          onClick={() => setDeleteFramed({ imsi: fr.imsi })}
                        >
                          <DeleteIcon fontSize="small" />
                        </IconButton>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Box>

      {/* Modals */}
      {isEditModalOpen && (
        <EditDataNetworkModal
          open
          onClose={() => setEditModalOpen(false)}
          onSuccess={() => {
            refetch();
            showSnackbar("Data network updated successfully.", "success");
          }}
          initialData={dataNetwork}
        />
      )}
      {isDeleteConfirmOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteConfirmOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the data network "${name}"? This action cannot be undone.`}
        />
      )}
      {staticModal && (
        <CreateStaticIpModal
          open
          onClose={() => setStaticModal(null)}
          onSuccess={() => {
            invalidateStaticIps();
            showSnackbar(
              staticModal.mode === "edit"
                ? "Static IP updated successfully."
                : "Static IP created successfully.",
              "success",
            );
          }}
          dataNetwork={name!}
          ipv4Pool={dataNetwork.ipv4_pool}
          ipv6Pool={dataNetwork.ipv6_pool}
          edit={
            staticModal.mode === "edit"
              ? {
                  imsi: staticModal.imsi,
                  ipVersion: staticModal.ipVersion,
                  address: staticModal.address,
                }
              : undefined
          }
        />
      )}
      {deleteStatic && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteStatic(null)}
          onConfirm={handleStaticDeleteConfirm}
          title="Confirm Deletion"
          description={`Remove the static IP ${deleteStatic.address} (${deleteStatic.ipVersion}) for subscriber ${deleteStatic.imsi}?`}
        />
      )}
      {framedModal && (
        <CreateFramedRouteModal
          open
          onClose={() => setFramedModal(null)}
          onSuccess={() => {
            invalidateFramed();
            showSnackbar(
              framedModal.mode === "edit"
                ? "Framed routes updated successfully."
                : "Framed routes created successfully.",
              "success",
            );
          }}
          dataNetwork={name!}
          edit={
            framedModal.mode === "edit"
              ? {
                  imsi: framedModal.imsi,
                  ipv4: framedModal.ipv4,
                  ipv6: framedModal.ipv6,
                }
              : undefined
          }
        />
      )}
      {deleteFramed && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteFramed(null)}
          onConfirm={handleFramedDeleteConfirm}
          title="Confirm Deletion"
          description={`Remove all framed routes for subscriber ${deleteFramed.imsi}? This releases the subscriber's active session so it re-establishes without them.`}
        />
      )}
    </Box>
  );
};

export default DataNetworkDetail;
