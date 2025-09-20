"use client";

import React, { useMemo, useState } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
  Chip,
  Table,
  TableHead,
  TableBody,
  TableRow,
  TableCell,
  TableContainer,
  Paper,
  IconButton,
  Stack,
  Tabs,
  Tab,
} from "@mui/material";
import { Switch, FormControlLabel } from "@mui/material";
import { useTheme } from "@mui/material/styles";
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
import { useAuth } from "@/contexts/AuthContext";
import { useMutation, useQuery } from "@tanstack/react-query";

// Data Networks
import { listDataNetworks, deleteDataNetwork } from "@/queries/data_networks";
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

import type { DataNetwork, Route } from "@/types/types";

const MAX_WIDTH = 1400;

type TabKey = "data-networks" | "routes" | "nat";

export default function NetworkingPage() {
  const theme = useTheme();
  const { role, accessToken } = useAuth();
  const canEdit = role === "Admin" || role === "Network Manager";

  const [tab, setTab] = useState<TabKey>("data-networks");

  // ------------ Alerts (section-scoped) ------------
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

  // ------------ Data Networks ------------
  const {
    data: dataNetworks = [],
    isLoading: dnLoading,
    refetch: refetchDataNetworks,
  } = useQuery<DataNetwork[]>({
    queryKey: ["data-networks", accessToken],
    queryFn: () => listDataNetworks(accessToken || ""),
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
  });

  const [isCreateDNOpen, setCreateDNOpen] = useState(false);
  const [isEditDNOpen, setEditDNOpen] = useState(false);
  const [isDeleteDNOpen, setDeleteDNOpen] = useState(false);
  const [editDN, setEditDN] = useState<DataNetwork | null>(null);
  const [selectedDNName, setSelectedDNName] = useState<string | null>(null);

  const handleOpenCreateDN = () => setCreateDNOpen(true);
  const handleCloseCreateDN = () => setCreateDNOpen(false);
  const handleEditDN = (dn: DataNetwork) => {
    setEditDN(dn);
    setEditDNOpen(true);
  };
  const handleRequestDeleteDN = (name: string) => {
    setSelectedDNName(name);
    setDeleteDNOpen(true);
  };
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

  // ------------ Routes ------------
  const {
    data: routes = [],
    isLoading: rtLoading,
    refetch: refetchRoutes,
  } = useQuery<Route[]>({
    queryKey: ["routes", accessToken],
    queryFn: () => listRoutes(accessToken || ""),
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

  // ------------ NAT ------------
  type NatInfo = { enabled: boolean };
  const {
    data: natInfo,
    isLoading: natLoading,
    refetch: refetchNAT,
  } = useQuery<NatInfo>({
    queryKey: ["nat", accessToken],
    queryFn: () => getNATInfo(accessToken || ""),
    refetchOnWindowFocus: true,
  });

  const { mutate: setNATEnabled, isPending: natMutating } = useMutation<
    void,
    unknown,
    boolean,
    unknown
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

  // ------------ Render ------------
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

      {/* -------- Data Networks Tab -------- */}
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

          {dnLoading ? (
            <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
              <CircularProgress />
            </Box>
          ) : dataNetworks.length === 0 ? (
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
                      Data Networks
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

              <TableContainer
                component={Paper}
                elevation={0}
                sx={{ border: 1, borderColor: "divider" }}
              >
                <Table aria-label="data networks table" stickyHeader>
                  <TableHead>
                    <TableRow
                      sx={{
                        "& th": {
                          fontWeight: "bold",
                          backgroundColor:
                            theme.palette.mode === "light"
                              ? "#F5F5F5"
                              : "inherit",
                        },
                      }}
                    >
                      <TableCell>Name (DNN)</TableCell>
                      <TableCell>IP Pool</TableCell>
                      <TableCell>DNS</TableCell>
                      <TableCell sx={{ width: 100 }}>MTU</TableCell>
                      <TableCell sx={{ width: 120 }}>Sessions</TableCell>
                      {canEdit && <TableCell align="right">Actions</TableCell>}
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {dataNetworks.map((dn) => {
                      const sessionCount = Number(
                        (dn as unknown as { status?: { sessions?: number } })
                          .status?.sessions ?? 0,
                      );
                      return (
                        <TableRow key={dn.name} hover>
                          <TableCell>{dn.name}</TableCell>
                          <TableCell>{dn.ipPool}</TableCell>
                          <TableCell>{dn.dns}</TableCell>
                          <TableCell>{dn.mtu}</TableCell>
                          <TableCell>
                            <Chip
                              size="small"
                              label={sessionCount}
                              color={sessionCount > 0 ? "success" : "default"}
                              variant="filled"
                            />
                          </TableCell>
                          {canEdit && (
                            <TableCell align="right">
                              <IconButton
                                aria-label="edit"
                                onClick={() => handleEditDN(dn)}
                                size="small"
                              >
                                <EditIcon color="primary" />
                              </IconButton>
                              <IconButton
                                aria-label="delete"
                                onClick={() => handleRequestDeleteDN(dn.name)}
                                size="small"
                              >
                                <DeleteIcon color="primary" />
                              </IconButton>
                            </TableCell>
                          )}
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </TableContainer>
            </>
          )}
        </Box>
      )}

      {/* -------- Routes Tab -------- */}
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

              <TableContainer
                component={Paper}
                elevation={0}
                sx={{ border: 1, borderColor: "divider" }}
              >
                <Table aria-label="routes table" stickyHeader>
                  <TableHead>
                    <TableRow
                      sx={{
                        "& th": {
                          fontWeight: "bold",
                          backgroundColor:
                            theme.palette.mode === "light"
                              ? "#F5F5F5"
                              : "inherit",
                        },
                      }}
                    >
                      <TableCell>ID</TableCell>
                      <TableCell>Destination</TableCell>
                      <TableCell>Gateway</TableCell>
                      <TableCell>Interface</TableCell>
                      <TableCell>Metric</TableCell>
                      {canEdit && <TableCell align="right">Actions</TableCell>}
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {routes.map((route) => (
                      <TableRow key={route.id} hover>
                        <TableCell>{route.id}</TableCell>
                        <TableCell>{route.destination}</TableCell>
                        <TableCell>{route.gateway}</TableCell>
                        <TableCell>{route.interface}</TableCell>
                        <TableCell>{route.metric}</TableCell>
                        {canEdit && (
                          <TableCell align="right">
                            <IconButton
                              aria-label="delete"
                              onClick={() => handleRequestDeleteRoute(route.id)}
                              size="small"
                            >
                              <DeleteIcon color="primary" />
                            </IconButton>
                          </TableCell>
                        )}
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            </>
          )}
        </Box>
      )}

      {/* -------- NAT Tab -------- */}
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
              ipPool: "",
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
