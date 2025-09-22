"use client";

import React, { useMemo, useState } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  FormControlLabel,
  Alert,
  Collapse,
  Chip,
  IconButton,
  Stack,
  Tabs,
  Tab,
  Paper,
  Switch,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
import { useAuth } from "@/contexts/AuthContext";
import { useMutation, useQuery } from "@tanstack/react-query";

// Data Networks
import {
  listDataNetworks,
  deleteDataNetwork,
  type ListDataNetworksResponse,
  type APIDataNetwork,
} from "@/queries/data_networks";
import CreateDataNetworkModal from "@/components/CreateDataNetworkModal";
import EditDataNetworkModal from "@/components/EditDataNetworkModal";

// Routes
import { listRoutes, deleteRoute } from "@/queries/routes";
import CreateRouteModal from "@/components/CreateRouteModal";

// NAT (global)
import { getNATInfo, updateNATInfo } from "@/queries/nat";

// Shared UI
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";

import type { Route } from "@/types/types";

import {
  DataGrid,
  type GridColDef,
  GridActionsCellItem,
  type GridRenderCellParams,
  type GridPaginationModel,
} from "@mui/x-data-grid";

const MAX_WIDTH = 1400;

type TabKey = "data-networks" | "routes" | "nat";

export default function NetworkingPage() {
  const { role, accessToken } = useAuth();
  const canEdit = role === "Admin" || role === "Network Manager";

  const [tab, setTab] = useState<TabKey>("data-networks");

  // ---------------- Alerts ----------------
  const [dnAlert, setDnAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });
  const [rtAlert, setRtAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });
  const [natAlert, setNatAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });

  // ====================== Data Networks (SERVER PAGINATION) ======================
  const [dnPagination, setDnPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const {
    data: dnPage,
    isLoading: dnLoading,
    refetch: refetchDataNetworks,
  } = useQuery<ListDataNetworksResponse>({
    queryKey: [
      "data-networks",
      accessToken,
      dnPagination.page,
      dnPagination.pageSize,
    ],
    queryFn: () => {
      const pageOneBased = dnPagination.page + 1;
      return listDataNetworks(
        accessToken || "",
        pageOneBased,
        dnPagination.pageSize,
      );
    },
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
  });

  const dnRows: APIDataNetwork[] = dnPage?.items ?? [];
  const dnRowCount = dnPage?.total_count ?? 0;

  const [isCreateDNOpen, setCreateDNOpen] = useState(false);
  const [isEditDNOpen, setEditDNOpen] = useState(false);
  const [isDeleteDNOpen, setDeleteDNOpen] = useState(false);
  const [editDN, setEditDN] = useState<APIDataNetwork | null>(null);
  const [selectedDNName, setSelectedDNName] = useState<string | null>(null);

  const handleOpenCreateDN = () => setCreateDNOpen(true);
  const handleCloseCreateDN = () => setCreateDNOpen(false);
  const handleEditDN = (dn: APIDataNetwork) => {
    setEditDN(dn);
    setEditDNOpen(true);
  };
  const handleRequestDeleteDN = (name: string) => {
    setSelectedDNName(name);
    setDeleteDNOpen(true);
  };

  const outerTheme = useTheme();
  const gridTheme = useMemo(
    () =>
      createTheme(outerTheme, {
        palette: { DataGrid: { headerBg: "#F5F5F5" } },
      }),
    [outerTheme],
  );

  const handleConfirmDeleteDN = async () => {
    setDeleteDNOpen(false);
    if (!selectedDNName || !accessToken) return;
    try {
      await deleteDataNetwork(accessToken, selectedDNName);
      setDnAlert({
        message: `Data Network "${selectedDNName}" deleted successfully!`,
        severity: "success",
      });
      refetchDataNetworks();
    } catch (error: unknown) {
      setDnAlert({
        message: `Failed to delete data network "${selectedDNName}": ${String(error)}`,
        severity: "error",
      });
    } finally {
      setSelectedDNName(null);
    }
  };

  const dnDescription = useMemo(
    () =>
      "Manage the IP networks used by your subscribers. Data Network Names (DNNs) are used to identify different networks, and must be configured on the subscriber device to connect to the correct network.",
    [],
  );

  const dnColumns: GridColDef<APIDataNetwork>[] = useMemo(() => {
    return [
      { field: "name", headerName: "Name (DNN)", flex: 1, minWidth: 200 },
      { field: "ip_pool", headerName: "IP Pool", flex: 1, minWidth: 180 },
      { field: "dns", headerName: "DNS", flex: 1, minWidth: 180 },
      { field: "mtu", headerName: "MTU", width: 100 },
      {
        field: "sessions",
        headerName: "Sessions",
        width: 120,
        valueGetter: (_v, row) =>
          Number(
            (row as unknown as { status?: { sessions?: number } })?.status
              ?.sessions ?? 0,
          ),
        renderCell: (params: GridRenderCellParams<APIDataNetwork>) => {
          const count = Number(
            (params.row as unknown as { status?: { sessions?: number } })
              ?.status?.sessions ?? 0,
          );
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
      ...(canEdit
        ? [
            {
              field: "actions",
              headerName: "Actions",
              type: "actions",
              width: 120,
              sortable: false,
              disableColumnMenu: true,
              getActions: (params: { row: APIDataNetwork }) => [
                <GridActionsCellItem
                  key="edit"
                  icon={<EditIcon color="primary" />}
                  label="Edit"
                  onClick={() => handleEditDN(params.row)}
                />,
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon color="primary" />}
                  label="Delete"
                  onClick={() => handleRequestDeleteDN(params.row.name)}
                />,
              ],
            } as GridColDef<APIDataNetwork>,
          ]
        : []),
    ];
  }, [canEdit]);

  // ====================== Routes (unchanged) ======================
  const {
    data: routes = [],
    isLoading: rtLoading,
    refetch: refetchRoutes,
  } = useQuery<Route[]>({
    queryKey: ["routes", accessToken],
    queryFn: () => listRoutes(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
  });

  const [isCreateRouteOpen, setCreateRouteOpen] = useState(false);
  const [isDeleteRouteOpen, setDeleteRouteOpen] = useState(false);
  const [selectedRouteId, setSelectedRouteId] = useState<string | null>(null);

  const handleOpenCreateRoute = () => setCreateRouteOpen(true);
  const handleRequestDeleteRoute = (routeID: string) => {
    setSelectedRouteId(routeID);
    setDeleteRouteOpen(true);
  };
  const handleConfirmDeleteRoute = async () => {
    setDeleteRouteOpen(false);
    if (!selectedRouteId || !accessToken) return;

    const idNum = Number(selectedRouteId);
    if (Number.isNaN(idNum)) {
      setRtAlert({
        message: `Invalid route id "${selectedRouteId}".`,
        severity: "error",
      });
      setSelectedRouteId(null);
      return;
    }

    try {
      await deleteRoute(accessToken, idNum);
      setRtAlert({
        message: `Route "${selectedRouteId}" deleted successfully!`,
        severity: "success",
      });
      refetchRoutes();
    } catch (error: unknown) {
      setRtAlert({
        message: `Failed to delete route "${selectedRouteId}": ${String(error)}`,
        severity: "error",
      });
    } finally {
      setSelectedRouteId(null);
    }
  };

  const routesDescription = useMemo(
    () =>
      "Manage the routing table for subscriber traffic. Created routes are applied as kernel routes on the nodes running the UPF.",
    [],
  );

  // ====================== NAT (unchanged) ======================
  type NatInfo = { enabled: boolean };
  const {
    data: natInfo,
    isLoading: natLoading,
    refetch: refetchNAT,
  } = useQuery<NatInfo>({
    queryKey: ["nat", accessToken],
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
      setNatAlert({ message: "NAT updated", severity: "success" });
      refetchNAT();
    },
    onError: (error: unknown) => {
      setNatAlert({
        message: `Failed to update NAT: ${String(error)}`,
        severity: "error",
      });
    },
  });

  const natDescription = useMemo(
    () =>
      "Network Address Translation (NAT) simplifies networking as it lets subscribers use private IP addresses without requiring an external router. It uses Ella Core's N6 IP as the source for outbound traffic. Enabling NAT adds processing overhead and some niche protocols won't work (e.g., FTP active mode).",
    [],
  );

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
      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}>
        <Typography variant="h4" sx={{ mb: 1 }}>
          Networking
        </Typography>
        <Typography variant="body1" color="text.secondary" sx={{ mb: 2 }}>
          Configure networks and packet forwarding for UE traffic.
        </Typography>

        <Tabs
          value={tab}
          onChange={(_, v) => setTab(v as TabKey)}
          aria-label="Networking sections"
          sx={{ borderBottom: 1, borderColor: "divider" }}
        >
          <Tab value="data-networks" label="Data Networks" />
          <Tab value="routes" label="Routes" />
          <Tab value="nat" label="NAT" />
        </Tabs>
      </Box>

      {/* ================= Data Networks Tab (paginated) ================= */}
      {tab === "data-networks" && (
        <Box
          sx={{
            width: "100%",
            mt: 2,
            maxWidth: MAX_WIDTH,
            px: { xs: 2, sm: 4 },
          }}
        >
          <Collapse in={!!dnAlert.message}>
            <Alert
              severity={dnAlert.severity || "success"}
              onClose={() => setDnAlert({ message: "", severity: null })}
              sx={{ mb: 2 }}
            >
              {dnAlert.message}
            </Alert>
          </Collapse>

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
                  </Box>
                  {canEdit && (
                    <Button
                      variant="contained"
                      color="success"
                      onClick={handleOpenCreateDN}
                      sx={{ alignSelf: { xs: "stretch", sm: "auto" } }}
                    >
                      Create
                    </Button>
                  )}
                </Stack>
              </Box>
              <ThemeProvider theme={gridTheme}>
                <DataGrid<APIDataNetwork>
                  rows={dnRows}
                  columns={dnColumns}
                  getRowId={(row) => row.name}
                  loading={dnLoading}
                  paginationMode="server"
                  rowCount={dnRowCount}
                  paginationModel={dnPagination}
                  onPaginationModelChange={setDnPagination}
                  pageSizeOptions={[10, 25, 50, 100]}
                  sortingMode="server"
                  disableColumnMenu
                  disableRowSelectionOnClick
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
                    "& .MuiDataGrid-columnHeaderTitle": { fontWeight: "bold" },
                  }}
                />
              </ThemeProvider>
            </>
          )}
        </Box>
      )}

      {/* ---------------- Routes Tab (unchanged table) ---------------- */}
      {tab === "routes" && (
        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            px: { xs: 2, sm: 4 },
            mt: 2,
          }}
        >
          <Collapse in={!!rtAlert.message}>
            <Alert
              severity={rtAlert.severity || "success"}
              onClose={() => setRtAlert({ message: "", severity: null })}
              sx={{ mb: 2 }}
            >
              {rtAlert.message}
            </Alert>
          </Collapse>

          {rtLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
              <CircularProgress />
            </Box>
          ) : routes.length === 0 ? (
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
                      Routes
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {routesDescription}
                    </Typography>
                  </Box>
                  {canEdit && (
                    <Button
                      variant="contained"
                      color="success"
                      onClick={handleOpenCreateRoute}
                      sx={{ alignSelf: { xs: "stretch", sm: "auto" } }}
                    >
                      Create
                    </Button>
                  )}
                </Stack>
              </Box>

              {/* keep your existing table for routes if you prefer */}
              <Paper variant="outlined" sx={{ p: 1 }}>
                <table style={{ width: "100%" }}>
                  <thead>
                    <tr>
                      <th style={{ textAlign: "left", padding: 8 }}>ID</th>
                      <th style={{ textAlign: "left", padding: 8 }}>
                        Destination
                      </th>
                      <th style={{ textAlign: "left", padding: 8 }}>Gateway</th>
                      <th style={{ textAlign: "left", padding: 8 }}>
                        Interface
                      </th>
                      <th style={{ textAlign: "left", padding: 8 }}>Metric</th>
                      {canEdit && (
                        <th style={{ textAlign: "right", padding: 8 }}>
                          Actions
                        </th>
                      )}
                    </tr>
                  </thead>
                  <tbody>
                    {routes.map((route) => (
                      <tr key={route.id}>
                        <td style={{ padding: 8 }}>{route.id}</td>
                        <td style={{ padding: 8 }}>{route.destination}</td>
                        <td style={{ padding: 8 }}>{route.gateway}</td>
                        <td style={{ padding: 8 }}>{route.interface}</td>
                        <td style={{ padding: 8 }}>{route.metric}</td>
                        {canEdit && (
                          <td style={{ padding: 8, textAlign: "right" }}>
                            <IconButton
                              aria-label="delete"
                              onClick={() => handleRequestDeleteRoute(route.id)}
                              size="small"
                            >
                              <DeleteIcon color="primary" />
                            </IconButton>
                          </td>
                        )}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </Paper>
            </>
          )}
        </Box>
      )}

      {/* ---------------- NAT Tab (unchanged) ---------------- */}
      {tab === "nat" && (
        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            px: { xs: 2, sm: 4 },
            mt: 2,
          }}
        >
          <Collapse in={!!natAlert.message}>
            <Alert
              severity={natAlert.severity || "success"}
              onClose={() => setNatAlert({ message: "", severity: null })}
              sx={{ mb: 2 }}
            >
              {natAlert.message}
            </Alert>
          </Collapse>

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

      {/* ------------ Modals ------------ */}
      {isCreateDNOpen && (
        <CreateDataNetworkModal
          open
          onClose={handleCloseCreateDN}
          onSuccess={refetchDataNetworks}
        />
      )}
      {isEditDNOpen && (
        <EditDataNetworkModal
          open
          onClose={() => setEditDNOpen(false)}
          onSuccess={refetchDataNetworks}
          initialData={
            editDN || {
              name: "",
              ip_pool: "",
              dns: "",
              mtu: 1500,
            }
          }
        />
      )}
      {isDeleteDNOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteDNOpen(false)}
          onConfirm={handleConfirmDeleteDN}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the data network "${selectedDNName}"? This action cannot be undone.`}
        />
      )}

      {isCreateRouteOpen && (
        <CreateRouteModal
          open
          onClose={() => setCreateRouteOpen(false)}
          onSuccess={refetchRoutes}
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
    </Box>
  );
}
