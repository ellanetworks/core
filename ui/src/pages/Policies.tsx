import React, { useMemo, useState } from "react";
import { Box, Typography, Button, CircularProgress } from "@mui/material";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  DataGrid,
  type GridColDef,
  type GridRenderCellParams,
  type GridPaginationModel,
} from "@mui/x-data-grid";

import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  listPolicies,
  type APIPolicy,
  type ListPoliciesResponse,
} from "@/queries/policies";
import CreatePolicyModal from "@/components/CreatePolicyModal";
import EmptyState from "@/components/EmptyState";
import { useAuth } from "@/contexts/AuthContext";
import { MAX_WIDTH } from "@/utils/layout";
import { Link } from "react-router-dom";

const PolicyPage = () => {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin" || role === "Network Manager";

  const theme = useTheme();
  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: theme.palette.backgroundSubtle } },
      }),
    [theme],
  );

  const [pagination, setPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const queryClient = useQueryClient();
  const pageOneBased = pagination.page + 1;
  const { data: pageData, isLoading: loading } = useQuery<ListPoliciesResponse>(
    {
      queryKey: ["policies", pageOneBased, pagination.pageSize],
      queryFn: () =>
        listPolicies(accessToken || "", pageOneBased, pagination.pageSize),
      enabled: authReady && !!accessToken,
      placeholderData: (prev) => prev,
    },
  );

  const [isCreateModalOpen, setCreateModalOpen] = useState(false);

  const { showSnackbar } = useSnackbar();

  const descriptionText =
    "Define bitrate and priority levels for your subscribers.";

  const handleOpenCreateModal = () => setCreateModalOpen(true);

  const rows: APIPolicy[] = pageData?.items ?? [];
  const rowCount = pageData?.total_count ?? 0;

  const columns: GridColDef<APIPolicy>[] = useMemo(() => {
    return [
      {
        field: "name",
        headerName: "Name",
        flex: 1,
        minWidth: 180,
        renderCell: (params: GridRenderCellParams<APIPolicy>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Link
              to={`/policies/${params.row.name}`}
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
                {params.row.name}
              </Typography>
            </Link>
          </Box>
        ),
      },
      {
        field: "bitrate_uplink",
        headerName: "Bitrate (Up)",
        flex: 1,
        minWidth: 160,
      },
      {
        field: "bitrate_downlink",
        headerName: "Bitrate (Down)",
        flex: 1,
        minWidth: 160,
      },
      { field: "var5qi", headerName: "5QI", width: 90 },
      { field: "arp", headerName: "ARP", width: 110 },
      {
        field: "data_network_name",
        headerName: "Data Network",
        flex: 1,
        minWidth: 160,
        renderCell: (params: GridRenderCellParams<APIPolicy>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Link
              to={`/networking/data-networks/${params.row.data_network_name}`}
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
                {params.row.data_network_name}
              </Typography>
            </Link>
          </Box>
        ),
      },
    ];
  }, [theme]);

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
      ) : rowCount === 0 ? (
        <EmptyState
          primaryText="No policy found."
          secondaryText="Create a new policy to control QoS and routing for subscribers."
          extraContent={
            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>
          }
          button={canEdit}
          buttonText="Create"
          onCreate={handleOpenCreateModal}
          readOnlyHint="Ask an administrator to create a policy."
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
            <Typography variant="h4">Policies ({rowCount})</Typography>

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
            <ThemeProvider theme={gridTheme}>
              <DataGrid<APIPolicy>
                rows={rows}
                columns={columns}
                getRowId={(row) => row.name}
                paginationMode="server"
                rowCount={rowCount}
                paginationModel={pagination}
                onPaginationModelChange={setPagination}
                pageSizeOptions={[10, 25, 50, 100]}
                disableRowSelectionOnClick
                disableColumnMenu
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
        <CreatePolicyModal
          open
          onClose={() => setCreateModalOpen(false)}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ["policies"] });
            showSnackbar("Policy created successfully.", "success");
          }}
        />
      )}
    </Box>
  );
};

export default PolicyPage;
