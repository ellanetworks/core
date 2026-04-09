import React, { useEffect, useMemo, useState } from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
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
import { useQuery } from "@tanstack/react-query";
import {
  getPolicy,
  deletePolicy,
  type APIPolicy,
  type PolicyRule,
} from "@/queries/policies";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import EditPolicyModal from "@/components/EditPolicyModal";
import PolicyRulesModal from "@/components/PolicyRulesModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";
import {
  formatProtocol,
  PROTOCOL_CHIP_COLORS,
  UPLINK_COLOR,
  DOWNLINK_COLOR,
} from "@/utils/formatters";

const labelCellSx = { fontWeight: 600, width: "35%" } as const;
const valueCellSx = { width: "65%", textAlign: "right" } as const;

const GRID_HEIGHT = 421;

interface RuleRow extends PolicyRule {
  index: number;
}

const PolicyDetail: React.FC = () => {
  const { profileName, policyName: name } = useParams<{
    profileName: string;
    policyName: string;
  }>();
  const navigate = useNavigate();
  const { role, accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const theme = useTheme();
  const canEdit = role === "Admin" || role === "Network Manager";

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: theme.palette.backgroundSubtle } },
      }),
    [theme],
  );

  const [isEditModalOpen, setEditModalOpen] = useState(false);
  const [isDeleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [rulesModalDirection, setRulesModalDirection] = useState<
    "uplink" | "downlink" | null
  >(null);

  useEffect(() => {
    if (authReady && !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const {
    data: policy,
    isLoading,
    error,
    refetch,
  } = useQuery<APIPolicy>({
    queryKey: ["policy", name],
    queryFn: () => getPolicy(accessToken!, name!),
    enabled: authReady && !!accessToken && !!name,
    refetchInterval: 5000,
  });

  const handleDeleteConfirm = async () => {
    if (!name || !accessToken) return;

    try {
      await deletePolicy(accessToken, name);
      setDeleteConfirmOpen(false);
      showSnackbar(`Policy "${name}" deleted successfully.`, "success");
      navigate(`/profiles/${profileName}`);
    } catch (err) {
      setDeleteConfirmOpen(false);
      showSnackbar(
        `Failed to delete policy: ${err instanceof Error ? err.message : "Unknown error"}`,
        "error",
      );
    }
  };

  const ruleColumns: GridColDef<RuleRow>[] = useMemo(
    () => [
      {
        field: "index",
        headerName: "#",
        width: 50,
        renderCell: (params: GridRenderCellParams<RuleRow>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Typography variant="body2">{params.row.index}</Typography>
          </Box>
        ),
      },
      {
        field: "action",
        headerName: "Action",
        width: 90,
        renderCell: (params: GridRenderCellParams<RuleRow>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Chip
              size="small"
              label={params.row.action.toUpperCase()}
              color={params.row.action === "allow" ? "success" : "error"}
              variant="outlined"
            />
          </Box>
        ),
      },
      {
        field: "protocol",
        headerName: "Protocol",
        width: 110,
        renderCell: (params: GridRenderCellParams<RuleRow>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Chip
              size="small"
              label={
                params.row.protocol === 0
                  ? "any"
                  : formatProtocol(params.row.protocol)
              }
              variant="outlined"
              sx={{
                borderColor:
                  PROTOCOL_CHIP_COLORS[params.row.protocol] || "divider",
                color:
                  PROTOCOL_CHIP_COLORS[params.row.protocol] || "text.primary",
              }}
            />
          </Box>
        ),
      },
      {
        field: "remote_prefix",
        headerName: "Remote Prefix",
        flex: 1,
        minWidth: 140,
        renderCell: (params: GridRenderCellParams<RuleRow>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Typography
              variant="body2"
              sx={{
                fontFamily: "monospace",
                ...(params.row.remote_prefix
                  ? {}
                  : { color: "text.secondary" }),
              }}
            >
              {params.row.remote_prefix || "any"}
            </Typography>
          </Box>
        ),
      },
      {
        field: "ports",
        headerName: "Ports",
        width: 110,
        renderCell: (params: GridRenderCellParams<RuleRow>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Typography variant="body2">
              {params.row.port_low === 0 && params.row.port_high === 0
                ? "any"
                : params.row.port_low === params.row.port_high
                  ? String(params.row.port_low)
                  : `${params.row.port_low}-${params.row.port_high}`}
            </Typography>
          </Box>
        ),
      },
      {
        field: "description",
        headerName: "Description",
        flex: 1,
        minWidth: 120,
        renderCell: (params: GridRenderCellParams<RuleRow>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Typography
              variant="body2"
              noWrap
              sx={params.row.description ? {} : { color: "text.secondary" }}
            >
              {params.row.description || "—"}
            </Typography>
          </Box>
        ),
      },
    ],
    [],
  );

  if (!authReady || isLoading) {
    return (
      <Box
        sx={{
          pt: 6,
          pb: 4,
          maxWidth: MAX_WIDTH,
          mx: "auto",
          px: PAGE_PADDING_X,
        }}
      >
        <Skeleton variant="text" width={320} height={48} sx={{ mb: 3 }} />
        <Skeleton variant="rounded" height={220} />
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            gap: 3,
            mt: 3,
          }}
        >
          <Skeleton variant="rounded" height={300} />
          <Skeleton variant="rounded" height={300} />
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
          {error instanceof Error ? error.message : "Failed to load policy."}
        </Typography>
        <Button
          variant="outlined"
          component={RouterLink}
          to={`/profiles/${profileName}`}
        >
          Back to Profile
        </Button>
      </Box>
    );
  }

  if (!policy) return null;

  const uplinkRules: RuleRow[] = (policy.rules?.uplink ?? []).map(
    (rule, idx) => ({ ...rule, index: idx + 1 }),
  );
  const downlinkRules: RuleRow[] = (policy.rules?.downlink ?? []).map(
    (rule, idx) => ({ ...rule, index: idx + 1 }),
  );

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
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
            <Typography
              component={RouterLink}
              to={`/profiles/${profileName}`}
              variant="h4"
              sx={{
                color: "text.secondary",
                textDecoration: "none",
                "&:hover": { textDecoration: "underline" },
              }}
            >
              {profileName}
            </Typography>
            <Typography
              component="span"
              variant="h4"
              sx={{ color: "text.secondary", mx: 1 }}
            >
              /
            </Typography>
            <Typography
              component="span"
              variant="h4"
              sx={{ color: "text.secondary" }}
            >
              Policies
            </Typography>
            <Typography
              component="span"
              variant="h4"
              sx={{ color: "text.secondary", mx: 1 }}
            >
              /
            </Typography>
            <Typography component="span" variant="h4">
              {policy.name}
            </Typography>
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            A policy defines the QoS parameters and network rules applied to a
            subscriber's session on a specific data network.
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

      {/* Configuration Card (full width) */}
      <Card
        variant="outlined"
        sx={{ display: "flex", flexDirection: "column" }}
      >
        <CardContent sx={{ flex: 1, display: "flex", flexDirection: "column" }}>
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
            sx={{
              "& tr:last-child td": { borderBottom: "none" },
            }}
          >
            <TableBody>
              <TableRow>
                <TableCell sx={labelCellSx}>
                  <Tooltip
                    title="The data network this policy applies to"
                    arrow
                    placement="top"
                  >
                    <span>Data Network</span>
                  </Tooltip>
                </TableCell>
                <TableCell sx={valueCellSx}>
                  <RouterLink
                    to={`/networking/data-networks/${policy.data_network_name}`}
                    style={{ textDecoration: "none" }}
                  >
                    <Typography
                      variant="body2"
                      sx={{
                        color: theme.palette.link,
                        textDecoration: "underline",
                        "&:hover": { textDecoration: "underline" },
                      }}
                    >
                      {policy.data_network_name}
                    </Typography>
                  </RouterLink>
                </TableCell>
              </TableRow>
              <TableRow>
                <TableCell sx={labelCellSx}>
                  <Tooltip
                    title="The network slice this policy belongs to"
                    arrow
                    placement="top"
                  >
                    <span>Slice</span>
                  </Tooltip>
                </TableCell>
                <TableCell sx={valueCellSx}>
                  <RouterLink
                    to="/networking/slices"
                    style={{ textDecoration: "none" }}
                  >
                    <Typography
                      variant="body2"
                      sx={{
                        color: theme.palette.link,
                        textDecoration: "underline",
                        "&:hover": { textDecoration: "underline" },
                      }}
                    >
                      {policy.slice_name}
                    </Typography>
                  </RouterLink>
                </TableCell>
              </TableRow>
              <TableRow>
                <TableCell sx={labelCellSx}>
                  <Tooltip
                    title="Maximum uplink bitrate for a single PDU session (Session AMBR). Enforced by Ella Core."
                    arrow
                    placement="top"
                  >
                    <span>Session Bitrate Uplink</span>
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
                      {policy.session_ambr_uplink}
                    </Typography>
                  </Box>
                </TableCell>
              </TableRow>
              <TableRow>
                <TableCell sx={labelCellSx}>
                  <Tooltip
                    title="Maximum downlink bitrate for a single PDU session (Session AMBR). Enforced by Ella Core."
                    arrow
                    placement="top"
                  >
                    <span>Session Bitrate Downlink</span>
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
                      {policy.session_ambr_downlink}
                    </Typography>
                  </Box>
                </TableCell>
              </TableRow>
              <TableRow>
                <TableCell sx={labelCellSx}>
                  <Tooltip
                    title="5G QoS Identifier. The radio uses this to set scheduling behavior (latency budget, error rate, priority). Only non-GBR classes are supported."
                    arrow
                    placement="top"
                  >
                    <span>5QI</span>
                  </Tooltip>
                </TableCell>
                <TableCell sx={valueCellSx}>{policy.var5qi}</TableCell>
              </TableRow>
              <TableRow>
                <TableCell sx={labelCellSx}>
                  <Tooltip
                    title="Allocation and Retention Priority. Used by the radio at session setup for admission control and pre-emption decisions. 1 = highest, 15 = lowest. Has no effect on traffic once the session is established."
                    arrow
                    placement="top"
                  >
                    <span>ARP</span>
                  </Tooltip>
                </TableCell>
                <TableCell sx={valueCellSx}>{policy.arp}</TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* Network Rules */}
      <Box sx={{ mt: 3 }}>
        <Typography variant="h6" sx={{ mb: 0.5 }}>
          Network Rules
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Control which traffic is allowed or denied for subscribers using this
          policy. Rules are evaluated in order — the first match wins.
        </Typography>
      </Box>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
          gap: 3,
        }}
      >
        {/* Uplink Rules */}
        <Box>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              mb: 1,
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                Uplink
              </Typography>
              <NorthIcon sx={{ fontSize: 16, color: UPLINK_COLOR }} />
            </Box>
            {canEdit && (
              <IconButton
                size="small"
                color="primary"
                onClick={() => setRulesModalDirection("uplink")}
                aria-label="Edit uplink rules"
              >
                <EditIcon fontSize="small" />
              </IconButton>
            )}
          </Box>
          <ThemeProvider theme={gridTheme}>
            <DataGrid<RuleRow>
              rows={uplinkRules}
              columns={ruleColumns}
              getRowId={(row) => row.index}
              disableColumnMenu
              disableRowSelectionOnClick
              density="compact"
              hideFooter
              sx={{
                height: GRID_HEIGHT,
                border: 1,
                borderColor: "divider",
                "& .MuiDataGrid-cell": {
                  borderBottom: "1px solid",
                  borderColor: "divider",
                },
              }}
            />
          </ThemeProvider>
        </Box>

        {/* Downlink Rules */}
        <Box>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              mb: 1,
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                Downlink
              </Typography>
              <SouthIcon sx={{ fontSize: 16, color: DOWNLINK_COLOR }} />
            </Box>
            {canEdit && (
              <IconButton
                size="small"
                color="primary"
                onClick={() => setRulesModalDirection("downlink")}
                aria-label="Edit downlink rules"
              >
                <EditIcon fontSize="small" />
              </IconButton>
            )}
          </Box>
          <ThemeProvider theme={gridTheme}>
            <DataGrid<RuleRow>
              rows={downlinkRules}
              columns={ruleColumns}
              getRowId={(row) => row.index}
              disableColumnMenu
              disableRowSelectionOnClick
              density="compact"
              hideFooter
              sx={{
                height: GRID_HEIGHT,
                border: 1,
                borderColor: "divider",
                "& .MuiDataGrid-cell": {
                  borderBottom: "1px solid",
                  borderColor: "divider",
                },
              }}
            />
          </ThemeProvider>
        </Box>
      </Box>

      {/* Modals */}
      {isEditModalOpen && (
        <EditPolicyModal
          open
          onClose={() => setEditModalOpen(false)}
          onSuccess={() => {
            refetch();
            showSnackbar("Policy updated successfully.", "success");
          }}
          initialData={policy}
        />
      )}
      {rulesModalDirection && (
        <PolicyRulesModal
          open
          onClose={() => setRulesModalDirection(null)}
          onSuccess={() => {
            refetch();
            showSnackbar(
              `${rulesModalDirection === "uplink" ? "Uplink" : "Downlink"} rules updated successfully.`,
              "success",
            );
          }}
          policy={policy}
          direction={rulesModalDirection}
        />
      )}
      {isDeleteConfirmOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteConfirmOpen(false)}
          onConfirm={handleDeleteConfirm}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the policy "${name}"? This action cannot be undone.`}
        />
      )}
    </Box>
  );
};

export default PolicyDetail;
