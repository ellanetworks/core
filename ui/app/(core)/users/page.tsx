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
import { listUsers, deleteUser } from "@/queries/users";
import CreateUserModal from "@/components/CreateUserModal";
import EditUserModal from "@/components/EditUserModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie"


interface UserData {
  email: string;
}

const User = () => {
  const [cookies, setCookie, removeCookie] = useCookies(['user_token']);

  const [users, setUsers] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
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

  const handleModalSuccess = () => {
    fetchUsers();
    setAlert({ message: "User created successfully!" });
  };

  const handleEditClick = (user: any) => {
    const mappedUser = {
      email: user.email,
    };

    setEditData(mappedUser);
    setEditModalOpen(true);
  };

  const handleEditModalClose = () => {
    setEditModalOpen(false);
    setEditData(null);
  };

  const handleEditSuccess = () => {
    fetchUsers();
    setAlert({ message: "User updated successfully!" });
  };

  const handleDeleteClick = (userName: string) => {
    setSelectedUser(userName);
    setConfirmationOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (selectedUser) {
      try {
        await deleteUser(cookies.user_token, selectedUser);
        setAlert({
          message: `User "${selectedUser}" deleted successfully!`,
        });
        fetchUsers();
      } catch (error) {
        console.error("Error deleting user:", error);
        setAlert({
          message: `Failed to delete user "${selectedUser}".`,
        });
      } finally {
        setSelectedUser(null);
      }
    }
  };

  const handleConfirmationClose = () => {
    setConfirmationOpen(false);
    setSelectedUser(null);
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
      <Box sx={{ width: "60%" }}>
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
      {!loading && users.length > 0 && (
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
            Users
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
      ) : users.length === 0 ? (
        <EmptyState
          primaryText="No user found."
          secondaryText="Create a new user."
          buttonText="Create"
          onCreate={handleOpenCreateModal}
        />
      ) : (
        <Box
          sx={{
            width: "60%",
            overflowX: "auto",
          }}
        >
          <TableContainer component={Paper}>
            <Table sx={{ minWidth: 900 }} aria-label="user table">
              <TableHead>
                <TableRow>
                  <TableCell>Email</TableCell>
                  <TableCell align="right">Actions</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {users.map((user) => (
                  <TableRow
                    key={user.email}
                    sx={{ "&:last-child td, &:last-child th": { border: 0 } }}
                  >
                    <TableCell component="th" scope="row">
                      {user.email}
                    </TableCell>
                    <TableCell align="right">
                      <IconButton
                        aria-label="edit"
                        onClick={() => handleEditClick(user)}
                      >
                        <EditIcon />
                      </IconButton>
                      <IconButton
                        aria-label="delete"
                        onClick={() => handleDeleteClick(user.email)}
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
      <CreateUserModal
        open={isCreateModalOpen}
        onClose={handleCloseCreateModal}
        onSuccess={handleModalSuccess}
      />
      <EditUserModal
        open={isEditModalOpen}
        onClose={handleEditModalClose}
        onSuccess={handleEditSuccess}
        initialData={
          editData || {
            email: "",
          }
        }
      />
      <DeleteConfirmationModal
        open={isConfirmationOpen}
        onClose={handleConfirmationClose}
        onConfirm={handleDeleteConfirm}
        title="Confirm Deletion"
        description={`Are you sure you want to delete the user "${selectedUser}"? This action cannot be undone.`}
      />
    </Box>
  );
};

export default User;
