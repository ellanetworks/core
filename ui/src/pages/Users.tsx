// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Box, Typography, Button } from "@mui/material";
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
import QueryState from "@/components/QueryState";
import { useTheme } from "@mui/material/styles";
import { useAuth } from "@/contexts/AuthContext";
import { Link } from "react-router-dom";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const UserPage: React.FC = () => {
  const { accessToken, authReady } = useAuth();
  const theme = useTheme();

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const { showSnackbar } = useSnackbar();

  const queryClient = useQueryClient();
  const pageOneBased = paginationModel.page + 1;
  const usersQuery = useQuery<ListUsersResponse>({
    queryKey: ["users", pageOneBased, paginationModel.pageSize],
    queryFn: () =>
      listUsers(accessToken || "", pageOneBased, paginationModel.pageSize),
    enabled: authReady && !!accessToken,
    placeholderData: (prev) => prev,
  });

  const knownCount = usersQuery.data?.total_count;

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
    [theme.palette.link],
  );

  const descriptionText =
    "Manage user accounts. Users can have different roles with varying levels of access to the Ella Core UI and API.";

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <Box sx={{ mb: 3, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="h4" component="h1">
          {knownCount === undefined ? "Users" : `Users (${knownCount})`}
        </Typography>
        <Typography variant="body1" color="textSecondary">
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

      <QueryState
        query={usersQuery}
        resource="users"
        isEmpty={(data) => (data.total_count ?? 0) === 0}
        empty={
          <EmptyState
            primaryText="No users yet"
            secondaryText="Create a user to get started."
          />
        }
      >
        {(data) => (
          <DataGrid<APIUser>
            rows={data.items ?? []}
            columns={columns}
            getRowId={(row) => row.email}
            paginationMode="server"
            rowCount={data.total_count ?? 0}
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
        )}
      </QueryState>

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
