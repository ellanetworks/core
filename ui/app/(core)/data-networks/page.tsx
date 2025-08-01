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
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
import { listDataNetworks, deleteDataNetwork } from "@/queries/data_networks";
import CreateDataNetworkModal from "@/components/CreateDataNetworkModal";
import EditDataNetworkModal from "@/components/EditDataNetworkModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";
import { useAuth } from "@/contexts/AuthContext";
import { DataNetwork } from "@/types/types";

const DataNetworkPage = () => {
  const { role } = useAuth();
  const [cookies] = useCookies(["user_token"]);
  const [dataNetworks, setDataNetworks] = useState<DataNetwork[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [editData, setEditData] = useState<DataNetwork | null>(null);
  const [selectedDataNetwork, setSelectedDataNetwork] = useState<string | null>(
    null,
  );
  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({
    message: "",
    severity: null,
  });

  const fetchDataNetworks = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listDataNetworks(cookies.user_token);
      setDataNetworks(data);
    } catch (error) {
      console.error("Error fetching data networks:", error);
    } finally {
      setLoading(false);
    }
  }, [cookies.user_token]);

  useEffect(() => {
    fetchDataNetworks();
  }, [fetchDataNetworks]);

  const handleOpenCreateModal = () => setCreateModalOpen(true);
  const handleCloseCreateModal = () => setCreateModalOpen(false);

  const handleEditClick = (dataNetwork: DataNetwork) => {
    setEditData(dataNetwork);
    setEditModalOpen(true);
  };

  const handleDeleteClick = (dataNetworkName: string) => {
    setSelectedDataNetwork(dataNetworkName);
    setConfirmationOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (selectedDataNetwork) {
      try {
        await deleteDataNetwork(cookies.user_token, selectedDataNetwork);
        setAlert({
          message: `Data Network "${selectedDataNetwork}" deleted successfully!`,
          severity: "success",
        });
        fetchDataNetworks();
      } catch (error) {
        setAlert({
          message: `Failed to delete data network "${selectedDataNetwork}": ${error}`,
          severity: "error",
        });
      } finally {
        setSelectedDataNetwork(null);
      }
    }
  };

  const baseColumns: GridColDef[] = [
    { field: "name", headerName: "Name (DNN)", flex: 1 },
    { field: "ipPool", headerName: "IP Pool", flex: 1 },
    { field: "dns", headerName: "DNS", flex: 1 },
    { field: "mtu", headerName: "MTU", type: "number", flex: 1 },
  ];

  if (role === "Admin" || role === "Network Manager") {
    baseColumns.push({
      field: "actions",
      headerName: "Actions",
      type: "actions",
      flex: 1,
      getActions: (params) => [
        <IconButton
          key="edit"
          aria-label="edit"
          onClick={() => handleEditClick(params.row)}
        >
          <EditIcon />
        </IconButton>,
        <IconButton
          key="delete"
          aria-label="delete"
          onClick={() => handleDeleteClick(params.row.name)}
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
      ) : dataNetworks.length === 0 ? (
        <EmptyState
          primaryText="No data network found."
          secondaryText="Create a new data network in order to add subscribers to the network."
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
              Data Networks ({dataNetworks.length})
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
              rows={dataNetworks}
              columns={baseColumns}
              disableRowSelectionOnClick
              getRowId={(row) => row.name}
            />
          </Box>
        </>
      )}
      <CreateDataNetworkModal
        open={isCreateModalOpen}
        onClose={handleCloseCreateModal}
        onSuccess={fetchDataNetworks}
      />
      <EditDataNetworkModal
        open={isEditModalOpen}
        onClose={() => setEditModalOpen(false)}
        onSuccess={fetchDataNetworks}
        initialData={
          editData || {
            name: "",
            ipPool: "",
            dns: "",
            mtu: 1500,
          }
        }
      />
      <DeleteConfirmationModal
        open={isConfirmationOpen}
        onClose={() => setConfirmationOpen(false)}
        onConfirm={handleDeleteConfirm}
        title="Confirm Deletion"
        description={`Are you sure you want to delete the data network "${selectedDataNetwork}"? This action cannot be undone.`}
      />
    </Box>
  );
};

export default DataNetworkPage;
