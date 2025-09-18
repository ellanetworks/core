"use client";

import React, { useState } from "react";
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
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
import { listDataNetworks, deleteDataNetwork } from "@/queries/data_networks";
import CreateDataNetworkModal from "@/components/CreateDataNetworkModal";
import EditDataNetworkModal from "@/components/EditDataNetworkModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useAuth } from "@/contexts/AuthContext";
import { DataNetwork } from "@/types/types";
import { useQuery } from "@tanstack/react-query";

const MAX_WIDTH = 1400;

const DataNetworkPage = () => {
  const { accessToken, authReady, role } = useAuth();

  const {
    data: dataNetworks = [],
    isLoading,
    refetch,
  } = useQuery({
    queryKey: ["data-networks", accessToken],
    queryFn: () => listDataNetworks(accessToken || ""),
    enabled: authReady && !!accessToken,
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
  });
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
  const canEdit = role === "Admin" || role === "Network Manager";

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
    if (!selectedDataNetwork || !accessToken) return;
    try {
      await deleteDataNetwork(accessToken, selectedDataNetwork);
      setAlert({
        message: `Data Network "${selectedDataNetwork}" deleted successfully!`,
        severity: "success",
      });
      refetch();
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

  const descriptionText =
    "Manage the IP networks used by your subscribers. Data Network Names (DNNs) are used to identify different networks, and must be configured on the subscriber device to connect to the correct network.";

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

      {isLoading ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : dataNetworks.length === 0 ? (
        <EmptyState
          primaryText="No data network found."
          secondaryText="Create a new data network in order to add subscribers to the network."
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
            <Typography variant="h4">Data Networks</Typography>

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
              sx={{
                border: 1,
                borderColor: "divider",
              }}
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
                    const sessionCount = Number(dn?.status?.sessions ?? 0);
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
                              onClick={() => handleEditClick(dn)}
                              size="small"
                            >
                              <EditIcon color="primary" />
                            </IconButton>
                            <IconButton
                              aria-label="delete"
                              onClick={() => handleDeleteClick(dn.name)}
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
          </Box>
        </>
      )}
      {isCreateModalOpen && (
        <CreateDataNetworkModal
          open
          onClose={handleCloseCreateModal}
          onSuccess={refetch}
        />
      )}
      {isEditModalOpen && (
        <EditDataNetworkModal
          open
          onClose={() => setEditModalOpen(false)}
          onSuccess={refetch}
          initialData={
            editData || {
              name: "",
              ipPool: "",
              dns: "",
              mtu: 1500,
            }
          }
        />
      )}
      {isConfirmationOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setConfirmationOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the data network "${selectedDataNetwork}"? This action cannot be undone.`}
        />
      )}
    </Box>
  );
};

export default DataNetworkPage;
