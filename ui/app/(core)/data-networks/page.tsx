"use client";

import React, { useCallback, useState, useEffect, useMemo } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import { DataGrid, GridColDef, GridActionsCellItem } from "@mui/x-data-grid";
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
import { listDataNetworks, deleteDataNetwork } from "@/queries/data_networks";
import CreateDataNetworkModal from "@/components/CreateDataNetworkModal";
import EditDataNetworkModal from "@/components/EditDataNetworkModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";
import { useAuth } from "@/contexts/AuthContext";
import { DataNetwork } from "@/types/types";

const MAX_WIDTH = 1400;

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

  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));
  const canEdit = role === "Admin" || role === "Network Manager";

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
    if (!selectedDataNetwork) return;
    try {
      await deleteDataNetwork(cookies.user_token, selectedDataNetwork);
      setAlert({
        message: `Data Network "${selectedDataNetwork}" deleted successfully!`,
        severity: "success",
      });
      fetchDataNetworks();
    } catch (error) {
      setAlert({
        message: `Failed to delete data network "${selectedDataNetwork}": ${
          error instanceof Error ? error.message : "Unknown error"
        }`,
        severity: "error",
      });
    } finally {
      setSelectedDataNetwork(null);
    }
  };

  const columns: GridColDef[] = useMemo(() => {
    const cols: GridColDef[] = [
      { field: "name", headerName: "Name (DNN)", flex: 1, minWidth: 180 },
      { field: "ipPool", headerName: "IP Pool", flex: 1, minWidth: 160 },
      { field: "dns", headerName: "DNS", flex: 0.8, minWidth: 140 },
      {
        field: "mtu",
        headerName: "MTU",
        type: "number",
        flex: 0.4,
        minWidth: 90,
      },
    ];

    if (canEdit) {
      cols.push({
        field: "actions",
        headerName: "Actions",
        type: "actions",
        width: isSmDown ? 80 : 140,
        sortable: false,
        disableColumnMenu: true,
        getActions: (params) =>
          isSmDown
            ? [
                <GridActionsCellItem
                  key="edit"
                  icon={<EditIcon />}
                  label="Edit"
                  onClick={() => handleEditClick(params.row)}
                />,
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon />}
                  label="Delete"
                  onClick={() => handleDeleteClick(params.row.name)}
                  showInMenu
                />,
              ]
            : [
                <GridActionsCellItem
                  key="edit"
                  icon={<EditIcon />}
                  label="Edit"
                  onClick={() => handleEditClick(params.row)}
                />,
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon />}
                  label="Delete"
                  onClick={() => handleDeleteClick(params.row.name)}
                />,
              ],
      });
    }

    return cols;
  }, [canEdit, isSmDown]);

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
      {/* Alerts */}
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
      ) : dataNetworks.length === 0 ? (
        <EmptyState
          primaryText="No data network found."
          secondaryText="Create a new data network in order to add subscribers to the network."
          button={canEdit}
          buttonText="Create"
          onCreate={handleOpenCreateModal}
        />
      ) : (
        <>
          {/* Header */}
          <Box
            sx={{
              width: "100%",
              maxWidth: MAX_WIDTH,
              px: { xs: 2, sm: 4 },
              mb: 3,
              display: "flex",
              flexDirection: { xs: "column", sm: "row" },
              justifyContent: "space-between",
              alignItems: { xs: "flex-start", sm: "center" },
              gap: 2,
            }}
          >
            <Typography variant="h4">
              Data Networks ({dataNetworks.length})
            </Typography>
            {canEdit && (
              <Button
                variant="contained"
                color="success"
                onClick={handleOpenCreateModal}
                sx={{ maxWidth: 200, width: "100%" }}
              >
                Create
              </Button>
            )}
          </Box>

          {/* Grid */}
          <Box sx={{ width: "100%", maxWidth: MAX_WIDTH }}>
            <DataGrid
              rows={dataNetworks}
              columns={columns}
              getRowId={(row) => row.name}
              disableRowSelectionOnClick
              density="compact"
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

      {/* Modals */}
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
