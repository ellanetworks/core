// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useState } from "react";
import { Box, Typography, Button } from "@mui/material";
import AccessChip from "@/components/AccessChip";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useTheme } from "@mui/material/styles";
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
import QueryState from "@/components/QueryState";
import { useAuth } from "@/contexts/AuthContext";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";
import { Link } from "react-router-dom";

const ProfilesPage: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const canEdit = role === "Admin" || role === "Network Manager";

  const theme = useTheme();
  const [pagination, setPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const queryClient = useQueryClient();
  const pageOneBased = pagination.page + 1;
  const profilesQuery = useQuery<ListProfilesResponse>({
    queryKey: ["profiles", pageOneBased, pagination.pageSize],
    queryFn: () =>
      listProfiles(accessToken || "", pageOneBased, pagination.pageSize),
    enabled: authReady && !!accessToken,
    placeholderData: (prev) => prev,
  });

  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const { showSnackbar } = useSnackbar();

  const descriptionText =
    "Profiles define subscriber-level bitrate limits and group the QoS policies applied to their sessions.";

  const knownCount = profilesQuery.data?.total_count;

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
      {
        field: "access",
        headerName: "Access",
        description: "Radio access technologies this profile permits (4G / 5G)",
        flex: 0.6,
        minWidth: 110,
        sortable: false,
        renderCell: (params: GridRenderCellParams<APIProfile>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              height: "100%",
              gap: 0.5,
            }}
          >
            <AccessChip label="4G" active={params.row.allow_4g} />
            <AccessChip label="5G" active={params.row.allow_5g} />
          </Box>
        ),
      },
    ];
  }, [theme]);

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <Box sx={{ mb: 3, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="h4" component="h1">
          {knownCount === undefined ? "Profiles" : `Profiles (${knownCount})`}
        </Typography>
        <Typography variant="body1" color="textSecondary">
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

      <QueryState
        query={profilesQuery}
        resource="profiles"
        isEmpty={(data) => (data.total_count ?? 0) === 0}
        empty={
          <EmptyState
            primaryText="No profiles yet"
            secondaryText={
              canEdit
                ? "Create a profile to get started."
                : "Ask an administrator to create a profile."
            }
          />
        }
      >
        {(data) => (
          <DataGrid<APIProfile>
            rows={data.items ?? []}
            columns={columns}
            getRowId={(row) => row.name}
            paginationMode="server"
            rowCount={data.total_count ?? 0}
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
        )}
      </QueryState>

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
