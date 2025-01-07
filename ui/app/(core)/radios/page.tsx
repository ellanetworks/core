"use client";

import React, { useState, useEffect } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
  IconButton,
} from "@mui/material";
import { DataGrid, GridColDef, GridRenderCellParams } from "@mui/x-data-grid";
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
import { listRadios, deleteRadio } from "@/queries/radios";
import CreateRadioModal from "@/components/CreateRadioModal";
import EditRadioModal from "@/components/EditRadioModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";

interface RadioData {
  name: string;
  tac: string;
}

const Radio = () => {
  const [cookies] = useCookies(["user_token"]);
  const [radios, setRadios] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [editData, setEditData] = useState<RadioData | null>(null);
  const [selectedRadio, setSelectedRadio] = useState<string | null>(null);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const fetchRadios = async () => {
    setLoading(true);
    try {
      const data = await listRadios(cookies.user_token);
      setRadios(data);
    } catch (error) {
      console.error("Error fetching radios:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchRadios();
  }, []);

  const handleOpenCreateModal = () => setCreateModalOpen(true);
  const handleCloseCreateModal = () => setCreateModalOpen(false);

  const handleEditClick = (radio: any) => {
    const mappedRadio = {
      name: radio.name,
      tac: radio.tac,
    };
    setEditData(mappedRadio);
    setEditModalOpen(true);
  };

  const handleEditModalClose = () => {
    setEditModalOpen(false);
    setEditData(null);
  };

  const handleDeleteClick = (radioName: string) => {
    setSelectedRadio(radioName);
    setConfirmationOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (selectedRadio) {
      try {
        await deleteRadio(cookies.user_token, selectedRadio);
        setAlert({
          message: `Radio "${selectedRadio}" deleted successfully!`,
        });
        fetchRadios();
      } catch (error) {
        console.error("Error deleting radio:", error);
        setAlert({
          message: `Failed to delete radio "${selectedRadio}".`,
        });
      } finally {
        setSelectedRadio(null);
      }
    }
  };

  const columns: GridColDef[] = [
    { field: "name", headerName: "Name", flex: 1 },
    { field: "tac", headerName: "TAC", flex: 1 },
    {
      field: "actions",
      headerName: "Actions",
      type: "actions",
      flex: 0.5,
      getActions: (params) => [
        <IconButton
          aria-label="edit"
          onClick={() => handleEditClick(params.row)}
        >
          <EditIcon />
        </IconButton>,
        <IconButton
          aria-label="delete"
          onClick={() => handleDeleteClick(params.row.name)}
        >
          <DeleteIcon />
        </IconButton>
      ],
    },
  ];

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
            severity="success"
            onClose={() => setAlert({ message: "" })}
            sx={{ marginBottom: 2 }}
          >
            {alert.message}
          </Alert>
        </Collapse>
      </Box>
      {loading ? (
        <Box sx={{ display: "flex", justifyContent: "center", alignItems: "center" }}>
          <CircularProgress />
        </Box>
      ) : radios.length === 0 ? (
        <EmptyState
          primaryText="No radio found."
          secondaryText="Create a new radio."
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
              Radios ({radios.length})
            </Typography>
            <Button variant="contained" color="success" onClick={handleOpenCreateModal}>
              Create
            </Button>
          </Box>
          <Box
            sx={{
              height: "80vh",
              width: "60%",
              "& .MuiDataGrid-root": {
                border: "none",
              },
              "& .MuiDataGrid-cell": {
                borderBottom: "none",
              },
              "& .MuiDataGrid-columnHeaders": {
                borderBottom: "none",
              },
              "& .MuiDataGrid-footerContainer": {
                borderTop: "none",
              },
            }}
          >
            <DataGrid
              rows={radios}
              columns={columns}
              getRowId={(row) => row.name}
              disableRowSelectionOnClick
            />
          </Box>
        </>
      )}
      <CreateRadioModal
        open={isCreateModalOpen}
        onClose={handleCloseCreateModal}
        onSuccess={fetchRadios}
      />
      <EditRadioModal
        open={isEditModalOpen}
        onClose={handleEditModalClose}
        onSuccess={fetchRadios}
        initialData={
          editData || {
            name: "",
            tac: "",
          }
        }
      />
      <DeleteConfirmationModal
        open={isConfirmationOpen}
        onClose={() => setConfirmationOpen(false)}
        onConfirm={handleDeleteConfirm}
        title="Confirm Deletion"
        description={`Are you sure you want to delete the radio "${selectedRadio}"? This action cannot be undone.`}
      />
    </Box>
  );
};

export default Radio;
