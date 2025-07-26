"use client";

import React, { useCallback, useState, useEffect } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
  IconButton,
} from "@mui/material";
import { DataGrid, GridColDef } from "@mui/x-data-grid";
import { Delete as DeleteIcon } from "@mui/icons-material";
import { listRoutes, deleteRoute } from "@/queries/routes";
import CreateRouteModal from "@/components/CreateRouteModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";
import { useAuth } from "@/contexts/AuthContext";
import { Route } from "@/types/types";

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
  const handleCloseCreateModal = () => setCreateModalOpen(false);

  const handleDeleteClick = (routeID: number) => {
    setSelectedRoute(routeID);
    setConfirmationOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (selectedRoute) {
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
    }
  };

  const baseColumns: GridColDef[] = [
    { field: "id", headerName: "Id", flex: 1 },
    { field: "destination", headerName: "Destination", flex: 1 },
    { field: "gateway", headerName: "Gateway", flex: 1 },
    { field: "interface", headerName: "Interface", flex: 1 },
    { field: "metric", headerName: "Metric", flex: 1 },
  ];

  if (role === "Admin" || role === "Network Manager") {
    baseColumns.push({
      field: "actions",
      headerName: "Actions",
      type: "actions",
      flex: 1,
      getActions: (params) => [
        <IconButton
          key="delete"
          aria-label="delete"
          onClick={() => handleDeleteClick(params.row.id)}
        >
          <DeleteIcon />
        </IconButton>,
      ],
    });
  }

  return (
    <Box
      sx={{
        height: "100vh",
        display: "flex",
        flexDirection: "column",
        justifyContent: "flex-start",
        alignItems: "center",
        paddingTop: 6,
        textAlign: "center",
      }}
    >
      <Box sx={{ width: "60%" }}>
        <Collapse in={!!alert.message}>
          <Alert
            severity={alert.severity || "success"}
            onClose={() => setAlert({ message: "", severity: null })}
            sx={{ marginBottom: 2 }}
          >
            {alert.message}
          </Alert>
        </Collapse>
      </Box>
      {loading ? (
        <Box
          sx={{
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
          }}
        >
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
              marginBottom: 4,
              width: "60%",
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <Typography variant="h4" component="h1" gutterBottom>
              Routes ({routes.length})
            </Typography>
            {(role === "Admin" || role === "Network Manager") && (
              <Button
                variant="contained"
                color="success"
                onClick={handleOpenCreateModal}
              >
                Create
              </Button>
            )}
          </Box>
          <Box
            sx={{
              height: "80vh",
              width: "60%",
              "& .MuiDataGrid-root": { border: "none" },
              "& .MuiDataGrid-cell": { borderBottom: "none" },
              "& .MuiDataGrid-columnHeaders": { borderBottom: "none" },
              "& .MuiDataGrid-footerContainer": { borderTop: "none" },
            }}
          >
            <DataGrid
              rows={routes}
              columns={baseColumns}
              disableRowSelectionOnClick
              getRowId={(row) => row.id}
            />
          </Box>
        </>
      )}
      <CreateRouteModal
        open={isCreateModalOpen}
        onClose={handleCloseCreateModal}
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
