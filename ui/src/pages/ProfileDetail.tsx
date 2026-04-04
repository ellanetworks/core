import React, { useEffect, useMemo, useState } from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  IconButton,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import {
  Edit as EditIcon,
  North as NorthIcon,
  South as SouthIcon,
} from "@mui/icons-material";
import { Link as RouterLink, useNavigate, useParams } from "react-router-dom";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  DataGrid,
  type GridColDef,
  type GridRenderCellParams,
} from "@mui/x-data-grid";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { getProfile, deleteProfile, type APIProfile } from "@/queries/profiles";
import {
  listPolicies,
  type APIPolicy,
  type ListPoliciesResponse,
} from "@/queries/policies";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import EditProfileModal from "@/components/EditProfileModal";
import CreatePolicyModal from "@/components/CreatePolicyModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { UPLINK_COLOR, DOWNLINK_COLOR } from "@/utils/formatters";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const labelCellSx = { fontWeight: 600, width: "35%" } as const;
const valueCellSx = { width: "65%", textAlign: "right" } as const;

const ProfileDetail: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();
  const { role, accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const theme = useTheme();
  const canEdit = role === "Admin" || role === "Network Manager";
  const queryClient = useQueryClient();

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: theme.palette.backgroundSubtle } },
      }),
    [theme],
  );

  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isDeleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [isCreatePolicyOpen, setCreatePolicyOpen] = useState(false);

  useEffect(() => {
    if (authReady && !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const {
    data: profile,
    isLoading,
    error,
    refetch,
  } = useQuery<APIProfile>({
    queryKey: ["profile", name],
    queryFn: () => getProfile(accessToken!, name!),
    enabled: authReady && !!accessToken && !!name,
    refetchInterval: 5000,
  });

  const { data: policiesData } = useQuery<ListPoliciesResponse>({
    queryKey: ["policies", "profile", name],
    queryFn: () => listPolicies(accessToken!, 1, 100, name!),
    enabled: authReady && !!accessToken && !!name,
    refetchInterval: 5000,
  });

  const policies: APIPolicy[] = policiesData?.items ?? [];

  const handleDeleteConfirm = async () => {
    if (!name || !accessToken) return;
    try {
      await deleteProfile(accessToken, name);
      setDeleteConfirmOpen(false);
      showSnackbar(`Profile "${name}" deleted successfully.`, "success");
      navigate("/profiles");
    } catch (err) {
      setDeleteConfirmOpen(false);
      showSnackbar(
        `Failed to delete profile: ${err instanceof Error ? err.message : "Unknown error"}`,
        "error",
      );
    }
  };

  const policyColumns: GridColDef<APIPolicy>[] = useMemo(
    () => [
      {
        field: "name",
        headerName: "Name",
        flex: 1,
        minWidth: 100,
        renderCell: (params: GridRenderCellParams<APIPolicy>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <RouterLink
              to={`/profiles/${name}/policies/${params.row.name}`}
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
            </RouterLink>
          </Box>
        ),
      },
      {
        field: "data_network_name",
        headerName: "Data Network",
        flex: 1,
        minWidth: 100,
      },
      {
        field: "slice_name",
        headerName: "Slice",
        flex: 1,
        minWidth: 100,
      },
      {
        field: "session_ambr_uplink",
        headerName: "Session Bitrate Uplink",
        description:
          "Per-session uplink bitrate cap (Session AMBR). Enforced by Ella Core.",
        flex: 0.8,
        minWidth: 100,
      },
      {
        field: "session_ambr_downlink",
        headerName: "Session Bitrate Downlink",
        description:
          "Per-session downlink bitrate cap (Session AMBR). Enforced by Ella Core.",
        flex: 0.8,
        minWidth: 100,
      },
      {
        field: "var5qi",
        headerName: "5QI",
        description: "5G QoS Identifier — radio scheduling class",
        width: 70,
      },
      {
        field: "arp",
        headerName: "ARP",
        description:
          "Allocation and Retention Priority — admission control at session setup (1 = highest)",
        width: 70,
      },
    ],
    [name, theme],
  );

  if (!authReady || isLoading) {
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
        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            mx: "auto",
            px: PAGE_PADDING_X,
          }}
        >
          <Skeleton variant="text" width={320} height={48} sx={{ mb: 3 }} />
          <Skeleton variant="rounded" height={220} />
          <Skeleton variant="rounded" height={300} sx={{ mt: 3 }} />
        </Box>
      </Box>
    );
  }

  if (error) {
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          mt: 6,
          gap: 2,
        }}
      >
        <Typography color="error">
          {error instanceof Error ? error.message : "Failed to load profile."}
        </Typography>
        <Button variant="outlined" component={RouterLink} to="/profiles">
          Back to Profiles
        </Button>
      </Box>
    );
  }

  if (!profile) return null;

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
      <Box
        sx={{
          width: "100%",
          maxWidth: MAX_WIDTH,
          mx: "auto",
          px: PAGE_PADDING_X,
        }}
      >
        {/* Header / Breadcrumb */}
        <Box
          sx={{
            display: "flex",
            flexDirection: { xs: "column", sm: "row" },
            alignItems: { xs: "flex-start", sm: "center" },
            gap: 2,
            mb: 3,
          }}
        >
          <Box sx={{ flex: 1 }}>
            <Typography
              variant="h4"
              sx={{ display: "flex", alignItems: "baseline", gap: 0 }}
            >
              <Typography
                component={RouterLink}
                to="/profiles"
                variant="h4"
                sx={{
                  color: "text.secondary",
                  textDecoration: "none",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                Profiles
              </Typography>
              <Typography
                component="span"
                variant="h4"
                sx={{ color: "text.secondary", mx: 1 }}
              >
                /
              </Typography>
              <Typography component="span" variant="h4">
                {profile.name}
              </Typography>
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
              A profile defines the aggregate bitrate limits for a subscriber
              and groups the QoS policies applied to their sessions.
            </Typography>
          </Box>
          {canEdit && (
            <Box sx={{ display: "flex", gap: 1 }}>
              <Button
                variant="outlined"
                color="error"
                onClick={() => setDeleteConfirmOpen(true)}
              >
                Delete
              </Button>
            </Box>
          )}
        </Box>

        {/* Configuration Card */}
        <Card
          variant="outlined"
          sx={{ display: "flex", flexDirection: "column" }}
        >
          <CardContent
            sx={{ flex: 1, display: "flex", flexDirection: "column" }}
          >
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                mb: 1.5,
              }}
            >
              <Typography variant="h6">Configuration</Typography>
              {canEdit && (
                <IconButton
                  size="small"
                  color="primary"
                  onClick={() => setEditModalOpen(true)}
                  aria-label="Edit configuration"
                >
                  <EditIcon fontSize="small" />
                </IconButton>
              )}
            </Box>
            <Table
              size="small"
              sx={{ "& tr:last-child td": { borderBottom: "none" } }}
            >
              <TableBody>
                <TableRow>
                  <TableCell sx={labelCellSx}>
                    <Tooltip
                      title="Aggregate uplink cap across all of this subscriber's sessions (UE-AMBR). Enforced by the radio."
                      arrow
                      placement="top"
                    >
                      <span>Bitrate Uplink</span>
                    </Tooltip>
                  </TableCell>
                  <TableCell sx={valueCellSx}>
                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "flex-end",
                        gap: 0.5,
                      }}
                    >
                      <NorthIcon sx={{ fontSize: 16, color: UPLINK_COLOR }} />
                      <Typography variant="body2">
                        {profile.ue_ambr_uplink}
                      </Typography>
                    </Box>
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell sx={labelCellSx}>
                    <Tooltip
                      title="Aggregate downlink cap across all of this subscriber's sessions (UE-AMBR). Enforced by the radio."
                      arrow
                      placement="top"
                    >
                      <span>Bitrate Downlink</span>
                    </Tooltip>
                  </TableCell>
                  <TableCell sx={valueCellSx}>
                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "flex-end",
                        gap: 0.5,
                      }}
                    >
                      <SouthIcon sx={{ fontSize: 16, color: DOWNLINK_COLOR }} />
                      <Typography variant="body2">
                        {profile.ue_ambr_downlink}
                      </Typography>
                    </Box>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        {/* Policies Table */}
        <Box sx={{ mt: 4 }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              mb: 2,
            }}
          >
            <Box>
              <Typography variant="h6">Policies ({policies.length})</Typography>
              <Typography variant="body2" color="text.secondary">
                QoS policies applied to subscriber sessions on each data
                network.
              </Typography>
            </Box>
            {canEdit && (
              <Button
                variant="contained"
                color="success"
                size="small"
                onClick={() => setCreatePolicyOpen(true)}
              >
                Add Policy
              </Button>
            )}
          </Box>
          <ThemeProvider theme={gridTheme}>
            <DataGrid<APIPolicy>
              rows={policies}
              columns={policyColumns}
              getRowId={(row) => row.name}
              disableRowSelectionOnClick
              disableColumnMenu
              hideFooter={policies.length <= 25}
              pageSizeOptions={[25, 50, 100]}
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
      </Box>

      {/* Modals */}
      {isEditModalOpen && (
        <EditProfileModal
          open
          onClose={() => setEditModalOpen(false)}
          onSuccess={() => {
            refetch();
            showSnackbar("Profile updated successfully.", "success");
          }}
          initialData={profile}
        />
      )}
      {isCreatePolicyOpen && (
        <CreatePolicyModal
          open
          profileName={name!}
          onClose={() => setCreatePolicyOpen(false)}
          onSuccess={() => {
            queryClient.invalidateQueries({
              queryKey: ["policies", "profile", name],
            });
            showSnackbar("Policy created successfully.", "success");
          }}
        />
      )}
      {isDeleteConfirmOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteConfirmOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the profile "${name}"? This action cannot be undone.`}
        />
      )}
    </Box>
  );
};

export default ProfileDetail;
