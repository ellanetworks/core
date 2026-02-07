import React, { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Alert,
  Collapse,
} from "@mui/material";
import {
  DataGrid,
  type GridColDef,
  GridActionsCellItem,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import DeleteIcon from "@mui/icons-material/Delete";
import EditIcon from "@mui/icons-material/Edit";
import PasswordIcon from "@mui/icons-material/Password";
import {
  listUsers,
  deleteUser,
  roleIDToLabel,
  type ListUsersResponse,
  type APIUser,
  RoleID,
} from "@/queries/users";
import CreateUserModal from "@/components/CreateUserModal";
import EditUserModal from "@/components/EditUserModal";
import EditUserPasswordModal from "@/components/EditUserPasswordModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import { useAuth } from "@/contexts/AuthContext";

const MAX_WIDTH = 1400;

const UserPage: React.FC = () => {
  const { accessToken, authReady } = useAuth();

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isEditPasswordModalOpen, setEditPasswordModalOpen] = useState(false);
  const [isConfirmationOpen, setConfirmationOpen] = useState(false);
  const [editData, setEditData] = useState<APIUser | null>(null);
  const [editPasswordData, setEditPasswordData] = useState<APIUser | null>(
    null,
  );
  const [selectedUser, setSelectedUser] = useState<string | null>(null); // email
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const outerTheme = useTheme();
  const gridTheme = useMemo(
    () =>
      createTheme(outerTheme, {
        palette: { DataGrid: { headerBg: "#F5F5F5" } },
      }),
    [outerTheme],
  );

  const queryClient = useQueryClient();
  const pageOneBased = paginationModel.page + 1;
  const { data: usersData, isLoading: loading } = useQuery<ListUsersResponse>({
    queryKey: ["users", pageOneBased, paginationModel.pageSize],
    queryFn: () =>
      listUsers(accessToken || "", pageOneBased, paginationModel.pageSize),
    enabled: authReady && !!accessToken,
  });

  const rows: APIUser[] = usersData?.items ?? [];
  const rowCount = usersData?.total_count ?? 0;

  const handleOpenCreateModal = () => setCreateModalOpen(true);

  const handleEditPasswordClick = (user: APIUser) => {
    setEditPasswordData(user);
    setEditPasswordModalOpen(true);
  };

  const handleEditClick = (user: APIUser) => {
    setEditData(user);
    setEditModalOpen(true);
  };

  const handleDeleteClick = (email: string) => {
    setSelectedUser(email);
    setConfirmationOpen(true);
  };

  const handleDeleteConfirm = async () => {
    setConfirmationOpen(false);
    if (!selectedUser || !accessToken) return;
    try {
      await deleteUser(accessToken, selectedUser);
      setAlert({ message: `User "${selectedUser}" deleted successfully!` });
      queryClient.invalidateQueries({ queryKey: ["users"] });
    } catch {
      setAlert({ message: `Failed to delete user "${selectedUser}".` });
    } finally {
      setSelectedUser(null);
    }
  };

  const columns: GridColDef<APIUser>[] = useMemo(
    () => [
      { field: "email", headerName: "Email", flex: 1, minWidth: 220 },
      {
        field: "roleID",
        headerName: "Role",
        flex: 0.6,
        minWidth: 120,
        valueGetter: (_v, row) => roleIDToLabel(row.role_id),
      },
      {
        field: "actions",
        headerName: "Actions",
        type: "actions",
        width: 200,
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

  const showEmpty = !loading && rowCount === 0 && (rows?.length ?? 0) === 0;

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

      {loading && rowCount === 0 ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : showEmpty ? (
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
            <Typography variant="h4">Users ({rowCount})</Typography>

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
              <DataGrid<APIUser>
                rows={rows}
                columns={columns}
                getRowId={(row) => row.email}
                loading={loading}
                paginationMode="server"
                rowCount={rowCount}
                paginationModel={paginationModel}
                onPaginationModelChange={setPaginationModel}
                pageSizeOptions={[10, 25, 50, 100]}
                sortingMode="server"
                disableColumnMenu
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
          onSuccess={() =>
            queryClient.invalidateQueries({ queryKey: ["users"] })
          }
        />
      )}

      {isEditModalOpen && (
        <EditUserModal
          open
          onClose={() => setEditModalOpen(false)}
          onSuccess={() =>
            queryClient.invalidateQueries({ queryKey: ["users"] })
          }
          initialData={editData || { email: "", role_id: RoleID.ReadOnly }}
        />
      )}

      {isEditPasswordModalOpen && (
        <EditUserPasswordModal
          open
          onClose={() => setEditPasswordModalOpen(false)}
          onSuccess={() =>
            queryClient.invalidateQueries({ queryKey: ["users"] })
          }
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
