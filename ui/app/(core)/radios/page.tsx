"use client";

import React, { useState, useEffect } from "react";
import {
  Box,
  Typography,
  TableContainer,
  Table,
  TableCell,
  TableRow,
  TableHead,
  TableBody,
  Paper,
  CircularProgress,
  Button,
  Alert,
  IconButton,
  Collapse,
} from "@mui/material";
import {
  Delete as DeleteIcon,
  Edit as EditIcon,
} from "@mui/icons-material";
import { listRadios, deleteRadio } from "@/queries/radios";
import CreateRadioModal from "@/components/CreateRadioModal";
import EditRadioModal from "@/components/EditRadioModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useRouter } from "next/navigation"
import { useCookies } from "react-cookie"


interface RadioData {
  name: string;
  tac: string;
}

const Radio = () => {
  const router = useRouter();
  const [cookies, setCookie, removeCookie] = useCookies(['user_token']);

  if (!cookies.user_token) {
    router.push("/login")
  }
  const [radios, setRadios] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
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

  const handleModalSuccess = () => {
    fetchRadios();
    setAlert({ message: "Radio created successfully!" });
  };

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

  const handleEditSuccess = () => {
    fetchRadios();
    setAlert({ message: "Radio updated successfully!" });
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

  const handleConfirmationClose = () => {
    setConfirmationOpen(false);
    setSelectedRadio(null);
  };

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
      <Box sx={{ width: "50%" }}>
        <Collapse in={!!alert.message}>
          <Alert
            severity={"success"}
            onClose={() => setAlert({ message: "" })}
            sx={{ marginBottom: 2 }}
          >
            {alert.message}
          </Alert>
        </Collapse>
      </Box>
      {!loading && radios.length > 0 && (
        <Box
          sx={{
            marginBottom: 4,
            width: "50%",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
          }}
        >
          <Typography variant="h4" component="h1" gutterBottom>
            Radios
          </Typography>
          <Button
            variant="contained"
            color="success"
            onClick={handleOpenCreateModal}
          >
            Create
          </Button>
        </Box>
      )}
      {loading ? (
        <Box
          sx={{
            height: "100vh",
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
          }}
        >
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
        <Box
          sx={{
            width: "50%",
            overflowX: "auto",
          }}
        >
          <TableContainer component={Paper}>
            <Table sx={{ minWidth: 900 }} aria-label="radio table">
              <TableHead>
                <TableRow>
                  <TableCell>Name</TableCell>
                  <TableCell align="right">TAC</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {radios.map((radio) => (
                  <TableRow
                    key={radio.name}
                    sx={{ "&:last-child td, &:last-child th": { border: 0 } }}
                  >
                    <TableCell component="th" scope="row">
                      {radio.name}
                    </TableCell>
                    <TableCell align="right">{radio.tac}</TableCell>
                    <TableCell align="right">
                      <IconButton
                        aria-label="edit"
                        onClick={() => handleEditClick(radio)}
                      >
                        <EditIcon />
                      </IconButton>
                      <IconButton
                        aria-label="delete"
                        onClick={() => handleDeleteClick(radio.name)}
                      >
                        <DeleteIcon />
                      </IconButton>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Box>
      )}
      <CreateRadioModal
        open={isCreateModalOpen}
        onClose={handleCloseCreateModal}
        onSuccess={handleModalSuccess}
      />
      <EditRadioModal
        open={isEditModalOpen}
        onClose={handleEditModalClose}
        onSuccess={handleEditSuccess}
        initialData={
          editData || {
            name: "",
            tac: "",
          }
        }
      />
      <DeleteConfirmationModal
        open={isConfirmationOpen}
        onClose={handleConfirmationClose}
        onConfirm={handleDeleteConfirm}
        title="Confirm Deletion"
        description={`Are you sure you want to delete the radio "${selectedRadio}"? This action cannot be undone.`}
      />
    </Box>
  );
};

export default Radio;
