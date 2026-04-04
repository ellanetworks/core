import React, { useEffect, useMemo, useState } from "react";
import {
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
  TableRow,
  Typography,
} from "@mui/material";
import { Edit as EditIcon } from "@mui/icons-material";
import { Link as RouterLink, useNavigate, useParams } from "react-router-dom";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  DataGrid,
  type GridColDef,
  type GridPaginationModel,
  type GridRenderCellParams,
} from "@mui/x-data-grid";
import { useQuery } from "@tanstack/react-query";
import {
  getDataNetwork,
  deleteDataNetwork,
  listIPAllocations,
  type APIDataNetwork,
  type APIIPAllocation,
} from "@/queries/data_networks";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import EditDataNetworkModal from "@/components/EditDataNetworkModal";
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

  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isDeleteConfirmOpen, setDeleteConfirmOpen] = useState(false);

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
    queryKey: ["ip-allocations", name, allocPage, allocPerPage],
    queryFn: () =>
      listIPAllocations(accessToken!, name!, allocPage, allocPerPage),
    enabled: authReady && !!accessToken && !!name,
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

  const allocationColumns: GridColDef<APIIPAllocation>[] = useMemo(
    () => [
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
        headerName: "PDU Session ID",
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
    ],
    [theme],
  );

  if (!authReady || isLoading) {
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
        <Box
          sx={{
            width: "100%",
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
  const ipAlloc = dataNetwork.ip_allocation;
  const poolSize = ipAlloc?.pool_size ?? 0;
  const allocated = ipAlloc?.allocated ?? 0;
  const utilPercent = poolSize > 0 ? (allocated / poolSize) * 100 : 0;

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
      <Box
        sx={{
          width: "100%",
          maxWidth: MAX_WIDTH,
          mx: "auto",
          px: PAGE_PADDING_X,
        }}
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
                    <TableCell sx={labelCellSx}>IP Pool</TableCell>
                    <TableCell sx={valueCellSx}>
                      <Typography
                        variant="body2"
                        sx={{ fontFamily: "monospace" }}
                      >
                        {dataNetwork.ip_pool}
                      </Typography>
                    </TableCell>
                  </TableRow>
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
                    <TableCell sx={labelCellSx}>IP Pool Utilization</TableCell>
                    <TableCell sx={valueCellSx}>
                      <Box
                        sx={{ display: "flex", alignItems: "center", gap: 1 }}
                      >
                        <LinearProgress
                          variant="determinate"
                          value={Math.min(utilPercent, 100)}
                          sx={{ flex: 1, height: 8, borderRadius: 4 }}
                        />
                        <Typography
                          variant="body2"
                          sx={{ whiteSpace: "nowrap" }}
                        >
                          {allocated.toLocaleString()} /{" "}
                          {poolSize.toLocaleString()}
                        </Typography>
                      </Box>
                    </TableCell>
                  </TableRow>
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </Box>

        {/* IP Allocations DataGrid */}
        <Box sx={{ mt: 3 }}>
          <Typography variant="h6" sx={{ mb: 1 }}>
            IP Allocations ({allocationRowCount.toLocaleString()})
          </Typography>
          {allocationRowCount === 0 ? (
            <TableContainer sx={tableContainerSx}>
              <Box sx={{ p: 3, textAlign: "center" }}>
                <Typography variant="body2" color="text.secondary">
                  No IP addresses are currently allocated in this pool.
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
    </Box>
  );
};

export default DataNetworkDetail;
