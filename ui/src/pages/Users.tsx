import React, { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Box, Typography, Button, CircularProgress } from "@mui/material";
import { useSnackbar } from "@/contexts/SnackbarContext";
import {
  DataGrid,
  type GridColDef,
  type GridRenderCellParams,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import {
  listUsers,
  roleIDToLabel,
  type ListUsersResponse,
  type APIUser,
} from "@/queries/users";
import CreateUserModal from "@/components/CreateUserModal";
import EmptyState from "@/components/EmptyState";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import { useAuth } from "@/contexts/AuthContext";
import { Link } from "react-router-dom";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const UserPage: React.FC = () => {
  const { accessToken, authReady } = useAuth();
  const theme = useTheme();

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: theme.palette.backgroundSubtle } },
      }),
    [theme],
  );

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const { showSnackbar } = useSnackbar();

  const queryClient = useQueryClient();
  const pageOneBased = paginationModel.page + 1;
  const { data: usersData, isLoading: loading } = useQuery<ListUsersResponse>({
    queryKey: ["users", pageOneBased, paginationModel.pageSize],
    queryFn: () =>
      listUsers(accessToken || "", pageOneBased, paginationModel.pageSize),
    enabled: authReady && !!accessToken,
    placeholderData: (prev) => prev,
  });

  const rows: APIUser[] = usersData?.items ?? [];
  const rowCount = usersData?.total_count ?? 0;

  const handleOpenCreateModal = () => setCreateModalOpen(true);

  const columns: GridColDef<APIUser>[] = useMemo(
    () => [
      {
        field: "email",
        headerName: "Email",
        flex: 1,
        minWidth: 220,
        renderCell: (params: GridRenderCellParams<APIUser>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Link
              to={`/users/${encodeURIComponent(params.row.email)}`}
              style={{ textDecoration: "none" }}
              onClick={(e: React.MouseEvent) => e.stopPropagation()}
            >
              <Typography
                variant="body2"
                sx={{
                  color: theme.palette.link,
                  textDecoration: "underline",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                {params.row.email}
              </Typography>
            </Link>
          </Box>
        ),
      },
      {
        field: "roleID",
        headerName: "Role",
        flex: 0.6,
        minWidth: 120,
        valueGetter: (_v, row) => roleIDToLabel(row.role_id),
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
              mx: "auto",
              px: PAGE_PADDING_X,
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
            sx={{
              width: "100%",
              maxWidth: MAX_WIDTH,
              mx: "auto",
              px: PAGE_PADDING_X,
            }}
          >
            <ThemeProvider theme={gridTheme}>
              <DataGrid<APIUser>
                rows={rows}
                columns={columns}
                getRowId={(row) => row.email}
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
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ["users"] });
            showSnackbar("User created successfully.", "success");
          }}
        />
      )}
    </Box>
  );
};

export default UserPage;
