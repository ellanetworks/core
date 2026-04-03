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
  listProfiles,
  type APIProfile,
  type ListProfilesResponse,
} from "@/queries/profiles";
import CreateProfileModal from "@/components/CreateProfileModal";
import EmptyState from "@/components/EmptyState";
import { useAuth } from "@/contexts/AuthContext";
import { MAX_WIDTH } from "@/utils/layout";
import { Link } from "react-router-dom";

const ProfilesPage: React.FC = () => {
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
  const { data: pageData, isLoading: loading } = useQuery<ListProfilesResponse>(
    {
      queryKey: ["profiles", pageOneBased, pagination.pageSize],
      queryFn: () =>
        listProfiles(accessToken || "", pageOneBased, pagination.pageSize),
      enabled: authReady && !!accessToken,
      placeholderData: (prev) => prev,
    },
  );

  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const { showSnackbar } = useSnackbar();

  const descriptionText =
    "Profiles define subscriber-level bitrate limits and group the QoS policies applied to their sessions.";

  const rows: APIProfile[] = pageData?.items ?? [];
  const rowCount = pageData?.total_count ?? 0;

  const columns: GridColDef<APIProfile>[] = useMemo(() => {
    return [
      {
        field: "name",
        headerName: "Name",
        flex: 1,
        minWidth: 180,
        renderCell: (params: GridRenderCellParams<APIProfile>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Link
              to={`/profiles/${params.row.name}`}
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
        field: "ue_ambr_uplink",
        headerName: "Bitrate Uplink",
        description:
          "Aggregate uplink cap across all of a subscriber's sessions (enforced by the radio)",
        flex: 1,
        minWidth: 160,
      },
      {
        field: "ue_ambr_downlink",
        headerName: "Bitrate Downlink",
        description:
          "Aggregate downlink cap across all of a subscriber's sessions (enforced by the radio)",
        flex: 1,
        minWidth: 160,
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
          primaryText="No profile found."
          secondaryText="Create a new profile to define subscriber-level bitrate limits and QoS policies."
          extraContent={
            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>
          }
          button={canEdit}
          buttonText="Create"
          onCreate={() => setCreateModalOpen(true)}
          readOnlyHint="Ask an administrator to create a profile."
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
            <Typography variant="h4">Profiles ({rowCount})</Typography>

            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>

            {canEdit && (
              <Button
                variant="contained"
                color="success"
                onClick={() => setCreateModalOpen(true)}
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
              <DataGrid<APIProfile>
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
        <CreateProfileModal
          open
          onClose={() => setCreateModalOpen(false)}
          onSuccess={() => {
            queryClient.invalidateQueries({ queryKey: ["profiles"] });
            showSnackbar("Profile created successfully.", "success");
          }}
        />
      )}
    </Box>
  );
};

export default ProfilesPage;
