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
import { DataGrid, GridColDef, GridRenderCellParams } from '@mui/x-data-grid';
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
import { listUsers, deleteUser } from "@/queries/users";
import CreateUserModal from "@/components/CreateUserModal";
import EditUserModal from "@/components/EditUserModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";

interface UserData {
  email: string;
}

const User = () => {
  const [cookies] = useCookies(["user_token"]);
  const [users, setUsers] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [editData, setEditData] = useState<UserData | null>(null);
  const [selectedUser, setSelectedUser] = useState<string | null>(null);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const fetchUsers = async () => {
    setLoading(true);
    try {
      const data = await listUsers(cookies.user_token);
      setUsers(data);
    } catch (error) {
      console.error("Error fetching users:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  const handleOpenCreateModal = () => setCreateModalOpen(true);
  const handleCloseCreateModal = () => setCreateModalOpen(false);

  const handleEditClick = (user: any) => {
    setEditData({ email: user.email });
    setEditModalOpen(true);
  };

  const handleEditModalClose = () => {
    setEditModalOpen(false);
    setEditData(null);
  };

  const handleDeleteClick = (email: string) => {
    setSelectedUser(email);
    setConfirmationOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (selectedUser) {
      try {
        await deleteUser(cookies.user_token, selectedUser);
        setAlert({ message: `User "${selectedUser}" deleted successfully!` });
        fetchUsers();
      } catch (error) {
        console.error("Error deleting user:", error);
        setAlert({ message: `Failed to delete user "${selectedUser}".` });
      } finally {
        setSelectedUser(null);
      }
    }
  };

  const columns: GridColDef[] = [
    { field: "email", headerName: "Email", flex: 1 },
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
      ) : users.length === 0 ? (
        <EmptyState
          primaryText="No user found."
          secondaryText="Create a new user."
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
              Users ({users.length})
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
              rows={users}
              columns={columns}
              getRowId={(row) => row.email}
              disableRowSelectionOnClick
            />
          </Box>
        </>
      )}
      <CreateUserModal
        open={isCreateModalOpen}
        onClose={handleCloseCreateModal}
        onSuccess={fetchUsers}
      />
      <EditUserModal
        open={isEditModalOpen}
        onClose={handleEditModalClose}
        onSuccess={fetchUsers}
        initialData={editData || { email: "" }}
      />
      <DeleteConfirmationModal
        open={isConfirmationOpen}
        onClose={() => setConfirmationOpen(false)}
        onConfirm={handleDeleteConfirm}
        title="Confirm Deletion"
        description={`Are you sure you want to delete the user "${selectedUser}"? This action cannot be undone.`}
      />
    </Box>
  );
};

export default User;
