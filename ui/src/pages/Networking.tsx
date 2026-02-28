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
  Stack,
  Tabs,
  Tab,
  Switch,
  IconButton,
  Tooltip,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
import { useAuth } from "@/contexts/AuthContext";
import { useFleet } from "@/contexts/FleetContext";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";

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
import {
  listRoutes,
  deleteRoute,
  type ListRoutesResponse,
  type APIRoute,
} from "@/queries/routes";
import CreateRouteModal from "@/components/CreateRouteModal";

// NAT
import { getNATInfo, updateNATInfo, type NatInfo } from "@/queries/nat";

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

const MAX_WIDTH = 1400;

type TabKey =
  | "data-networks"
  | "interfaces"
  | "routes"
  | "nat"
  | "flow-accounting";

export default function NetworkingPage() {
  const { role, accessToken } = useAuth();
  const { isFleetManaged } = useFleet();
  const canEdit =
    (role === "Admin" || role === "Network Manager") && !isFleetManaged;

  const [searchParams, setSearchParams] = useSearchParams();

  // Read initial tab from URL, fallback to default
  const initialTabFromUrl =
    (searchParams.get("tab") as TabKey) || "data-networks";

  const [tab, setTab] = useState<TabKey>(initialTabFromUrl);

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
  const [flowAccountingAlert, setFlowAccountingAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });
  const [interfacesAlert, setInterfacesAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });

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
              getActions: (p: { row: APIDataNetwork }) => [
                <GridActionsCellItem
                  key="edit"
                  icon={<EditIcon color="primary" />}
                  label="Edit"
                  onClick={() => handleEditDN(p.row)}
                />,
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon color="primary" />}
                  label="Delete"
                  onClick={() => handleRequestDeleteDN(p.row.name)}
                />,
              ],
            } as GridColDef<APIDataNetwork>,
          ]
        : []),
    ];
  }, [canEdit]);

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

  const interfacesDescription = useMemo(
    () =>
      "View the network interfaces used by Ella Core for control plane (N2), user plane (N3), external networks (N6), and the API endpoint. Interfaces are primarily configured in the Ella Core configuration file; this page reflects that configuration, with N3's external address as the only editable field.",
    [],
  );

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
      "Manage the routing table for subscriber traffic. Created routes are applied as kernel routes on the node running Ella Core.",
    [],
  );

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
      ...(canEdit
        ? [
            {
              field: "actions",
              headerName: "Actions",
              type: "actions",
              width: 100,
              sortable: false,
              disableColumnMenu: true,
              getActions: (p: { row: APIRoute }) => [
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon color="primary" />}
                  label="Delete"
                  onClick={() => handleRequestDeleteRoute(String(p.row.id))}
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
      setFlowAccountingAlert({
        message: "Flow accounting updated",
        severity: "success",
      });
      refetchFlowAccounting();
    },
    onError: (error: unknown) => {
      setFlowAccountingAlert({
        message: `Failed to update flow accounting: ${String(error)}`,
        severity: "error",
      });
    },
  });

  const flowAccountingDescription = useMemo(
    () =>
      "Flow accounting records per-flow network usage (source/destination IP and port, protocol, bytes, packets) for each subscriber session. Disabling flow accounting reduces processing overhead and stops collection of new flow data.",
    [],
  );

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
      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}>
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
                  sortingMode="server"
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
                    "& .MuiDataGrid-columnHeaderTitle": { fontWeight: "bold" },
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
            px: { xs: 2, sm: 4 },
            mt: 2,
          }}
        >
          <Collapse in={!!interfacesAlert.message}>
            <Alert
              severity={interfacesAlert.severity || "success"}
              onClose={() =>
                setInterfacesAlert({ message: "", severity: null })
              }
              sx={{ mb: 2 }}
            >
              {interfacesAlert.message}
            </Alert>
          </Collapse>

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
                  getRowId={(row) => row.id}
                  paginationMode="server"
                  rowCount={rtRowCount}
                  paginationModel={rtPagination}
                  onPaginationModelChange={setRtPagination}
                  pageSizeOptions={[10, 25, 50, 100]}
                  sortingMode="server"
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
                    "& .MuiDataGrid-columnHeaderTitle": { fontWeight: "bold" },
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

      {/* ================= Flow Accounting Tab ================= */}
      {tab === "flow-accounting" && (
        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            px: { xs: 2, sm: 4 },
            mt: 2,
          }}
        >
          <Collapse in={!!flowAccountingAlert.message}>
            <Alert
              severity={flowAccountingAlert.severity || "success"}
              onClose={() =>
                setFlowAccountingAlert({ message: "", severity: null })
              }
              sx={{ mb: 2 }}
            >
              {flowAccountingAlert.message}
            </Alert>
          </Collapse>

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
          onSuccess={refetchDataNetworks}
        />
      )}
      {isEditDNOpen && (
        <EditDataNetworkModal
          open
          onClose={() => setEditDNOpen(false)}
          onSuccess={refetchDataNetworks}
          initialData={editDN || { name: "", ip_pool: "", dns: "", mtu: 1500 }}
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

      {isEditN3Open && (
        <EditInterfaceN3Modal
          open
          onClose={() => setEditN3Open(false)}
          onSuccess={() => {
            setInterfacesAlert({
              message: "N3 external address updated.",
              severity: "success",
            });
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
