"use client";

import React, { useCallback, useEffect, useState } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
  Table,
  TableHead,
  TableBody,
  TableRow,
  TableCell,
  TableContainer,
  Paper,
  IconButton,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import { Delete as DeleteIcon } from "@mui/icons-material";
import { listRoutes, deleteRoute } from "@/queries/routes";
import CreateRouteModal from "@/components/CreateRouteModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";
import { useAuth } from "@/contexts/AuthContext";
import { Route } from "@/types/types";

const MAX_WIDTH = 1400;

const RoutePage = () => {
  const { role } = useAuth();
  const [cookies] = useCookies(["user_token"]);
  const [routes, setRoutes] = useState<Route[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [selectedRoute, setSelectedRoute] = useState<string | null>(null);

  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });

  const theme = useTheme();
  const canEdit = role === "Admin" || role === "Network Manager";

  const fetchRoutes = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listRoutes(cookies.user_token);
      setRoutes(data);
    } catch (error) {
      console.error("Error fetching routes:", error);
    } finally {
      setLoading(false);
    }
  }, [cookies.user_token]);

  useEffect(() => {
    fetchRoutes();
  }, [fetchRoutes]);

  const handleOpenCreateModal = () => setCreateModalOpen(true);

  const handleDeleteClick = (routeID: string) => {
    setSelectedRoute(routeID);
    setConfirmationOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (!selectedRoute) return;

    const idNum = Number(selectedRoute);
    if (Number.isNaN(idNum)) {
      setAlert({
        message: `Invalid route id "${selectedRoute}".`,
        severity: "error",
      });
      setSelectedRoute(null);
      return;
    }

    try {
      await deleteRoute(cookies.user_token, idNum);
      setAlert({
        message: `Route "${selectedRoute}" deleted successfully!`,
        severity: "success",
      });
      fetchRoutes();
    } catch (error) {
      setAlert({
        message: `Failed to delete route "${selectedRoute}": ${
          error instanceof Error ? error.message : String(error)
        }`,
        severity: "error",
      });
    } finally {
      setSelectedRoute(null);
    }
  };

  const descriptionText =
    "Manage the routing table for subscriber traffic. Created routes will be applied as Linux kernel routes on the system.";

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

      {loading ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : routes.length === 0 ? (
        <EmptyState
          primaryText="No route found."
          secondaryText="Create a new route in order for subscribers to access the network."
          extraContent={
            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>
          }
          button={canEdit}
          buttonText="Create"
          onCreate={handleOpenCreateModal}
        />
      ) : (
        <>
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
            <Typography variant="h4">Routes</Typography>

            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>

            {canEdit && (
              <Button
                variant="contained"
                color="success"
                onClick={handleOpenCreateModal}
                sx={{ maxWidth: 200 }}
              >
                Create
              </Button>
            )}
          </Box>

          <Box
            sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}
          >
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
                            onClick={() => handleDeleteClick(route.id)}
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
          </Box>
        </>
      )}
      {isCreateModalOpen && (
        <CreateRouteModal
          open
          onClose={() => setCreateModalOpen(false)}
          onSuccess={fetchRoutes}
        />
      )}
      {isConfirmationOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setConfirmationOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the route "${selectedRoute}"? This action cannot be undone.`}
        />
      )}
    </Box>
  );
};

export default RoutePage;
