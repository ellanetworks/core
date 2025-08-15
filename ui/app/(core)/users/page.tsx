"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
} from "@mui/material";
import { DataGrid, GridColDef, GridActionsCellItem } from "@mui/x-data-grid";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import PasswordIcon from "@mui/icons-material/Password";
import { listUsers, deleteUser } from "@/queries/users";
import CreateUserModal from "@/components/CreateUserModal";
import EditUserModal from "@/components/EditUserModal";
import EditUserPasswordModal from "@/components/EditUserPasswordModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";
import { RoleID, User, roleIDToLabel } from "@/types/types";
import { useTheme } from "@mui/material/styles";
import { ThemeProvider, createTheme } from "@mui/material/styles";

const MAX_WIDTH = 1400;

const UserPage = () => {
  const [cookies] = useCookies(["user_token"]);
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isEditPasswordModalOpen, setEditPasswordModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [editData, setEditData] = useState<User | null>(null);
  const [editPasswordData, setEditPasswordData] = useState<User | null>(null);
  const [selectedUser, setSelectedUser] = useState<string | null>(null);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const outerTheme = useTheme();

  const gridTheme = React.useMemo(
    () =>
      createTheme(outerTheme, {
        palette: {
          DataGrid: { headerBg: "#F5F5F5" },
        },
      }),
    [outerTheme],
  );

  const fetchUsers = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listUsers(cookies.user_token);
      setUsers(data);
    } catch (error) {
      console.error("Error fetching users:", error);
    } finally {
      setLoading(false);
    }
  }, [cookies.user_token]);

  useEffect(() => {
    fetchUsers();
  }, [fetchUsers]);

  const handleOpenCreateModal = () => setCreateModalOpen(true);
  const handleEditPasswordClick = (user: User) => {
    setEditPasswordData(user);
    setEditPasswordModalOpen(true);
  };
  const handleEditClick = (user: User) => {
    setEditData(user);
    setEditModalOpen(true);
  };
  const handleDeleteClick = (email: string) => {
    setSelectedUser(email);
    setConfirmationOpen(true);
  };
  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (!selectedUser) return;
    try {
      await deleteUser(cookies.user_token, selectedUser);
      setAlert({ message: `User "${selectedUser}" deleted successfully!` });
      fetchUsers();
    } catch {
      setAlert({ message: `Failed to delete user "${selectedUser}".` });
    } finally {
      setSelectedUser(null);
    }
  };

  const columns: GridColDef[] = useMemo(
    () => [
      { field: "email", headerName: "Email", flex: 1, minWidth: 220 },
      {
        field: "roleID",
        headerName: "Role",
        flex: 0.6,
        minWidth: 120,
        valueGetter: (_v, row) => roleIDToLabel(row.roleID),
      },
      {
        field: "actions",
        headerName: "Actions",
        type: "actions",
        width: 160,
        sortable: false,
        disableColumnMenu: true,
        getActions: (params) => [
          <GridActionsCellItem
            key="edit"
            icon={<EditIcon color="primary" />}
            label="Edit"
            onClick={() => handleEditClick(params.row)}
          />,
          <GridActionsCellItem
            key="password"
            icon={<PasswordIcon color="primary" />}
            label="Change Password"
            onClick={() => handleEditPasswordClick(params.row)}
          />,
          <GridActionsCellItem
            key="delete"
            icon={<DeleteIcon color="primary" />}
            label="Delete"
            onClick={() => handleDeleteClick(params.row.email)}
          />,
        ],
      },
    ],
    [],
  );

  const descriptionText =
    "Manage user accounts. Users can have different roles with varying levels of access to the Ella Core UI and API.";

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
            severity="success"
            onClose={() => setAlert({ message: "" })}
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
      ) : users.length === 0 ? (
        <EmptyState
          primaryText="No user found."
          secondaryText="Create a new user."
          extraContent={
            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>
          }
          button
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
            <Typography variant="h4">Users ({users.length})</Typography>

            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>

            <Button
              variant="contained"
              color="success"
              onClick={handleOpenCreateModal}
              sx={{ maxWidth: 200 }}
            >
              Create
            </Button>
          </Box>

          <Box
            sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}
          >
            <ThemeProvider theme={gridTheme}>
              <DataGrid
                rows={users}
                columns={columns}
                getRowId={(row) => row.email}
                disableRowSelectionOnClick
                sx={{
                  width: "100%",
                  border: 1,
                  borderColor: "divider",
                  "& .MuiDataGrid-cell": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-columnHeaders": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-footerContainer": {
                    borderTop: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-columnHeaderTitle": { fontWeight: "bold" },
                }}
              />
            </ThemeProvider>
          </Box>
        </>
      )}

      {isCreateModalOpen && (
        <CreateUserModal
          open
          onClose={() => setCreateModalOpen(false)}
          onSuccess={fetchUsers}
        />
      )}
      {isEditModalOpen && (
        <EditUserModal
          open
          onClose={() => setEditModalOpen(false)}
          onSuccess={fetchUsers}
          initialData={editData || { email: "", roleID: RoleID.ReadOnly }}
        />
      )}
      {isEditPasswordModalOpen && (
        <EditUserPasswordModal
          open
          onClose={() => setEditPasswordModalOpen(false)}
          onSuccess={fetchUsers}
          initialData={editPasswordData || { email: "" }}
        />
      )}
      {isConfirmationOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setConfirmationOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the user "${selectedUser}"? This action cannot be undone.`}
        />
      )}
    </Box>
  );
};

export default UserPage;
