"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
  IconButton,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import { DataGrid, GridColDef } from "@mui/x-data-grid";
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
  const [selectedRoute, setSelectedRoute] = useState<number | null>(null);
  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({
    message: "",
    severity: null,
  });

  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));

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

  const handleDeleteClick = (routeID: number) => {
    setSelectedRoute(routeID);
    setConfirmationOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (!selectedRoute) return;
    try {
      await deleteRoute(cookies.user_token, selectedRoute);
      setAlert({
        message: `Route "${selectedRoute}" deleted successfully!`,
        severity: "success",
      });
      fetchRoutes();
    } catch (error) {
      setAlert({
        message: `Failed to delete route "${selectedRoute}": ${error}`,
        severity: "error",
      });
    } finally {
      setSelectedRoute(null);
    }
  };

  const columns: GridColDef[] = useMemo(
    () => [
      { field: "id", headerName: "ID", minWidth: 90, width: 100 },
      {
        field: "destination",
        headerName: "Destination",
        flex: 1,
        minWidth: 220,
      },
      { field: "gateway", headerName: "Gateway", flex: 1, minWidth: 180 },
      {
        field: "interface",
        headerName: "Interface",
        minWidth: 140,
        width: 160,
      },
      { field: "metric", headerName: "Metric", minWidth: 100, width: 120 },
      ...(role === "Admin" || role === "Network Manager"
        ? [
            {
              field: "actions",
              headerName: "Actions",
              type: "actions",
              width: 80,
              getActions: (params) => [
                <IconButton
                  key="delete"
                  aria-label="delete"
                  onClick={() => handleDeleteClick(params.row.id)}
                >
                  <DeleteIcon />
                </IconButton>,
              ],
            } as GridColDef,
          ]
        : []),
    ],
    [role],
  );

  return (
    <Box
      sx={{
        minHeight: "100vh",
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
          button={role === "Admin" || role === "Network Manager"}
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
              flexDirection: { xs: "column", sm: "row" },
              justifyContent: "space-between",
              alignItems: { xs: "stretch", sm: "center" },
              gap: 2,
            }}
          >
            <Typography variant="h4">Routes ({routes.length})</Typography>
            {(role === "Admin" || role === "Network Manager") && (
              <Button
                variant="contained"
                color="success"
                onClick={handleOpenCreateModal}
                sx={{
                  maxWidth: "200px",
                  width: "100%",
                }}
              >
                Create
              </Button>
            )}
          </Box>

          {/* Grid */}
          <Box sx={{ width: "100%", maxWidth: MAX_WIDTH }}>
            <DataGrid
              rows={routes}
              columns={columns}
              getRowId={(row) => row.id}
              disableRowSelectionOnClick
              density="compact"
              columnVisibilityModel={{
                metric: !isSmDown,
              }}
              sx={{
                width: "100%",
                height: { xs: 460, sm: 560, md: 640 },
                border: "none",
                "& .MuiDataGrid-cell": { borderBottom: "none" },
                "& .MuiDataGrid-columnHeaders": { borderBottom: "none" },
                "& .MuiDataGrid-footerContainer": { borderTop: "none" },
              }}
            />
          </Box>
        </>
      )}

      <CreateRouteModal
        open={isCreateModalOpen}
        onClose={() => setCreateModalOpen(false)}
        onSuccess={fetchRoutes}
      />

      <DeleteConfirmationModal
        open={isConfirmationOpen}
        onClose={() => setConfirmationOpen(false)}
        onConfirm={handleDeleteConfirm}
        title="Confirm Deletion"
        description={`Are you sure you want to delete the route "${selectedRoute}"? This action cannot be undone.`}
      />
    </Box>
  );
};

export default RoutePage;
