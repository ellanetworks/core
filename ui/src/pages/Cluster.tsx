import React, { useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  Paper,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import Grid from "@mui/material/Grid";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import {
  DataGrid,
  type GridColDef,
  type GridRenderCellParams,
  GridActionsCellItem,
} from "@mui/x-data-grid";
import ArrowUpwardIcon from "@mui/icons-material/ArrowUpward";
import DeleteIcon from "@mui/icons-material/Delete";
import PowerSettingsNewIcon from "@mui/icons-material/PowerSettingsNew";
import PlayArrowIcon from "@mui/icons-material/PlayArrow";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { getStatus, type APIStatus } from "@/queries/status";
import {
  listClusterMembers,
  getAutopilotState,
  getClusterPKIState,
  promoteClusterMember,
  removeClusterMember,
  type ClusterMember,
  type AutopilotServer,
  type AutopilotState,
  type ClusterPKIState,
  type DrainResponse,
  type DrainState,
  type ResumeResponse,
} from "@/queries/cluster";
import AddClusterMemberModal from "@/components/AddClusterMemberModal";
import MintJoinTokenModal from "@/components/MintJoinTokenModal";
import DrainNodeModal from "@/components/DrainNodeModal";
import ResumeNodeModal from "@/components/ResumeNodeModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

type JoinedRow = ClusterMember & {
  id: number;
  autopilot?: AutopilotServer;
};

// CenteredCell wraps renderCell content so it vertically centers within
// the DataGrid row. MUI's default cell rendering centers text, but
// renderCell output is top-aligned unless the returned element provides
// its own full-height flex container.
const CenteredCell: React.FC<{ children: React.ReactNode }> = ({
  children,
}) => (
  <Box
    sx={{
      display: "flex",
      alignItems: "center",
      height: "100%",
      width: "100%",
    }}
  >
    {children}
  </Box>
);

function formatLastContact(ms: number): string {
  if (ms < 0) return "—";
  if (ms < 1000) return `${ms} ms`;
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  return `${m}m ${s % 60}s`;
}

function drainStateChip(state: DrainState, updatedAt?: string) {
  if (state === "drained") {
    const title = updatedAt
      ? `Drained at ${new Date(updatedAt).toLocaleString()}. Safe to remove.`
      : "Node is drained; safe to remove.";
    return (
      <Tooltip title={title}>
        <Chip label="Drained" size="small" color="error" variant="outlined" />
      </Tooltip>
    );
  }
  if (state === "draining") {
    return (
      <Tooltip title="Drain in progress — waiting for local sessions to clear.">
        <Chip label="Draining" size="small" color="warning" />
      </Tooltip>
    );
  }
  return <Chip label="Active" size="small" variant="outlined" />;
}

async function copyToClipboard(
  text: string,
  showSnackbar: (msg: string, sev: "success" | "error") => void,
) {
  if (!navigator.clipboard) {
    showSnackbar(
      "Clipboard API not available. Please use HTTPS or try a different browser.",
      "error",
    );
    return;
  }
  try {
    await navigator.clipboard.writeText(text);
    showSnackbar("Copied to clipboard.", "success");
  } catch {
    showSnackbar("Failed to copy.", "error");
  }
}

const CopyableText: React.FC<{ value: string }> = ({ value }) => {
  const { showSnackbar } = useSnackbar();
  if (!value) {
    return (
      <CenteredCell>
        <Typography variant="body2">—</Typography>
      </CenteredCell>
    );
  }
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        gap: 0.5,
        minWidth: 0,
        width: "100%",
        height: "100%",
      }}
    >
      <Typography
        variant="body2"
        sx={{
          fontFamily: "monospace",
          overflow: "hidden",
          textOverflow: "ellipsis",
        }}
        title={value}
      >
        {value}
      </Typography>
      <Tooltip title="Copy">
        <IconButton
          size="small"
          onClick={(e) => {
            e.stopPropagation();
            copyToClipboard(value, showSnackbar);
          }}
        >
          <ContentCopyIcon fontSize="inherit" />
        </IconButton>
      </Tooltip>
    </Box>
  );
};

const HealthBanner: React.FC<{
  autopilot: AutopilotState | undefined;
  autopilotError: boolean;
  versionsDiffer: boolean;
}> = ({ autopilot, autopilotError, versionsDiffer }) => {
  if (autopilotError && !autopilot) {
    return (
      <Alert severity="warning" sx={{ mb: 2 }}>
        Live cluster health is unavailable (no leader reachable, or autopilot is
        still converging after a recent leadership change).
      </Alert>
    );
  }

  if (!autopilot) return null;

  const ft = autopilot.failureTolerance;
  const severity: "success" | "warning" | "error" =
    !autopilot.healthy || ft < 0 ? "error" : ft === 0 ? "warning" : "success";

  const ftText =
    ft < 0
      ? "Quorum is at risk — an additional failure will stall writes."
      : ft === 0
        ? "Can lose 0 more nodes before writes stall."
        : `Can lose ${ft} more node${ft === 1 ? "" : "s"} before writes stall.`;

  return (
    <Stack spacing={1} sx={{ mb: 2 }}>
      <Alert severity={severity}>
        <Typography variant="body2" component="span" sx={{ fontWeight: 600 }}>
          Cluster is {autopilot.healthy ? "healthy" : "unhealthy"}.
        </Typography>{" "}
        {ftText}
      </Alert>
      {versionsDiffer && (
        <Alert severity="warning">
          Nodes are running different binary versions. This is expected during a
          rolling upgrade but should not persist.
        </Alert>
      )}
    </Stack>
  );
};

const ClusterPKIPanel: React.FC<{
  pki: ClusterPKIState | undefined;
  error: boolean;
}> = ({ pki, error }) => {
  if (error && !pki) return null;
  if (!pki) return null;

  const activeInt = pki.intermediates.find((i) => i.status === "active");
  const now = Math.floor(Date.now() / 1000);
  const daysUntilExpiry = activeInt?.notAfter
    ? Math.floor((activeInt.notAfter - now) / 86400)
    : null;

  const expirySeverity: "success" | "warning" | "error" | null =
    daysUntilExpiry === null
      ? null
      : daysUntilExpiry < 7
        ? "error"
        : daysUntilExpiry < 30
          ? "warning"
          : "success";

  return (
    <Paper variant="outlined" sx={{ p: 2, mt: 2 }}>
      <Typography variant="subtitle1" sx={{ mb: 1, fontWeight: 600 }}>
        Cluster PKI
      </Typography>
      <Stack direction="row" spacing={3} sx={{ flexWrap: "wrap", rowGap: 1 }}>
        <Box>
          <Typography variant="caption" color="textSecondary">
            Cluster ID
          </Typography>
          <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
            {pki.clusterID || "—"}
          </Typography>
        </Box>
        <Box>
          <Typography variant="caption" color="textSecondary">
            Roots
          </Typography>
          <Typography variant="body2">{pki.roots.length}</Typography>
        </Box>
        <Box>
          <Typography variant="caption" color="textSecondary">
            Intermediates
          </Typography>
          <Typography variant="body2">{pki.intermediates.length}</Typography>
        </Box>
        <Box>
          <Typography variant="caption" color="textSecondary">
            Revoked serials
          </Typography>
          <Typography variant="body2">{pki.revokedSerialCount}</Typography>
        </Box>
        {activeInt?.notAfter && expirySeverity && (
          <Box>
            <Typography variant="caption" color="textSecondary">
              Active intermediate expires
            </Typography>
            <Typography
              variant="body2"
              color={
                expirySeverity === "error"
                  ? "error.main"
                  : expirySeverity === "warning"
                    ? "warning.main"
                    : "text.primary"
              }
            >
              {new Date(activeInt.notAfter * 1000).toLocaleDateString()}
              {daysUntilExpiry !== null && ` (${daysUntilExpiry}d)`}
            </Typography>
          </Box>
        )}
      </Stack>
    </Paper>
  );
};

const ClusterPage: React.FC = () => {
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const queryClient = useQueryClient();
  const theme = useTheme();

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: theme.palette.backgroundSubtle } },
      }),
    [theme],
  );

  const [isAddOpen, setAddOpen] = useState(false);
  const [isMintOpen, setMintOpen] = useState(false);
  const [drainTarget, setDrainTarget] = useState<ClusterMember | null>(null);
  const [resumeTarget, setResumeTarget] = useState<ClusterMember | null>(null);
  const [removeTarget, setRemoveTarget] = useState<ClusterMember | null>(null);

  const statusQuery = useQuery<APIStatus>({
    queryKey: ["status"],
    queryFn: getStatus,
    refetchInterval: 5000,
  });

  const clusterEnabled = statusQuery.data?.cluster?.enabled ?? false;

  const membersQuery = useQuery<ClusterMember[]>({
    queryKey: ["cluster-members"],
    queryFn: () => listClusterMembers(accessToken || ""),
    enabled: authReady && !!accessToken && clusterEnabled,
    refetchInterval: 5000,
  });

  const autopilotQuery = useQuery<AutopilotState>({
    queryKey: ["cluster-autopilot"],
    queryFn: () => getAutopilotState(accessToken || ""),
    enabled: authReady && !!accessToken && clusterEnabled,
    refetchInterval: 2000,
    retry: false,
  });

  const pkiQuery = useQuery<ClusterPKIState>({
    queryKey: ["cluster-pki-state"],
    queryFn: () => getClusterPKIState(accessToken || ""),
    enabled: authReady && !!accessToken && clusterEnabled,
    refetchInterval: 30000,
    retry: false,
  });

  const members = membersQuery.data ?? [];
  const autopilot = autopilotQuery.data;

  const rows: JoinedRow[] = useMemo(() => {
    const apByNode = new Map<number, AutopilotServer>();
    for (const s of autopilot?.servers ?? []) {
      apByNode.set(s.nodeId, s);
    }
    return members.map((m) => ({
      ...m,
      id: m.nodeId,
      autopilot: apByNode.get(m.nodeId),
    }));
  }, [members, autopilot]);

  const leaderIndex = useMemo(
    () => autopilot?.servers.find((s) => s.isLeader)?.lastIndex ?? 0,
    [autopilot],
  );

  const versionsDiffer = useMemo(() => {
    const versions = new Set(
      members.map((m) => m.binaryVersion).filter((v) => v !== ""),
    );
    return versions.size > 1;
  }, [members]);

  const handlePromote = async (m: ClusterMember) => {
    if (!accessToken) return;
    try {
      await promoteClusterMember(accessToken, m.nodeId);
      showSnackbar(`Node ${m.nodeId} promoted to voter.`, "success");
      queryClient.invalidateQueries({ queryKey: ["cluster-members"] });
      queryClient.invalidateQueries({ queryKey: ["cluster-autopilot"] });
    } catch (err) {
      showSnackbar(
        `Failed to promote: ${err instanceof Error ? err.message : "unknown error"}`,
        "error",
      );
    }
  };

  const handleRemoveConfirm = async () => {
    if (!accessToken || !removeTarget) return;
    const target = removeTarget;
    setRemoveTarget(null);
    try {
      await removeClusterMember(accessToken, target.nodeId);
      showSnackbar(`Node ${target.nodeId} removed.`, "success");
      queryClient.invalidateQueries({ queryKey: ["cluster-members"] });
      queryClient.invalidateQueries({ queryKey: ["cluster-autopilot"] });
    } catch (err) {
      showSnackbar(
        `Failed to remove: ${err instanceof Error ? err.message : "unknown error"}`,
        "error",
      );
    }
  };

  const handleDrainSuccess = (result: DrainResponse) => {
    const parts: string[] = [];
    if (result.transferredLeadership) parts.push("leadership transferred");
    if (result.ransNotified > 0)
      parts.push(`${result.ransNotified} RAN(s) notified`);
    if (result.bgpStopped) parts.push("BGP stopped");
    const detail = parts.length > 0 ? ` (${parts.join(", ")})` : "";
    showSnackbar(`Drain ${result.state}${detail}.`, "success");
    queryClient.invalidateQueries({ queryKey: ["status"] });
    queryClient.invalidateQueries({ queryKey: ["cluster-members"] });
    queryClient.invalidateQueries({ queryKey: ["cluster-autopilot"] });
  };

  const handleResumeSuccess = (result: ResumeResponse) => {
    const suffix = result.bgpStarted ? " (BGP restarted)" : "";
    showSnackbar(`Node resumed${suffix}.`, "success");
    queryClient.invalidateQueries({ queryKey: ["cluster-members"] });
    queryClient.invalidateQueries({ queryKey: ["cluster-autopilot"] });
  };

  const selfNodeId = statusQuery.data?.cluster?.nodeId ?? 0;
  const currentLeaderNodeId = statusQuery.data?.cluster?.leaderNodeId ?? 0;

  const columns: GridColDef<JoinedRow>[] = useMemo(
    () => [
      {
        field: "nodeId",
        headerName: "Node ID",
        width: 240,
        renderCell: (p: GridRenderCellParams<JoinedRow>) => (
          <Stack
            direction="row"
            spacing={1}
            sx={{ alignItems: "center", height: "100%" }}
          >
            <Typography
              variant="body2"
              sx={{ fontWeight: p.row.isLeader ? 700 : 400 }}
            >
              {p.row.nodeId}
            </Typography>
            {p.row.isLeader && (
              <Chip label="Leader" color="primary" size="small" />
            )}
            {p.row.nodeId === selfNodeId && (
              <Chip label="This node" size="small" variant="outlined" />
            )}
          </Stack>
        ),
      },
      {
        field: "apiAddress",
        headerName: "API Address",
        flex: 1,
        minWidth: 200,
        renderCell: (p: GridRenderCellParams<JoinedRow>) => (
          <CopyableText value={p.row.apiAddress} />
        ),
      },
      {
        field: "suffrage",
        headerName: "Suffrage",
        width: 110,
        renderCell: (p: GridRenderCellParams<JoinedRow>) => (
          <CenteredCell>
            <Chip
              label={p.row.suffrage}
              size="small"
              color={p.row.suffrage === "voter" ? "primary" : "warning"}
              variant={p.row.suffrage === "voter" ? "filled" : "outlined"}
            />
          </CenteredCell>
        ),
      },
      {
        field: "drainState",
        headerName: "Drain",
        width: 110,
        renderCell: (p: GridRenderCellParams<JoinedRow>) => (
          <CenteredCell>
            {drainStateChip(p.row.drainState, p.row.drainUpdatedAt)}
          </CenteredCell>
        ),
      },
      {
        field: "binaryVersion",
        headerName: "Version",
        width: 130,
        renderCell: (p: GridRenderCellParams<JoinedRow>) => {
          if (!p.row.binaryVersion)
            return (
              <CenteredCell>
                <Typography variant="body2">—</Typography>
              </CenteredCell>
            );
          const skewed = versionsDiffer;
          return (
            <CenteredCell>
              <Chip
                label={p.row.binaryVersion}
                size="small"
                color={skewed ? "warning" : "default"}
                variant="outlined"
              />
            </CenteredCell>
          );
        },
      },
      {
        field: "healthy",
        headerName: "Healthy",
        width: 110,
        renderCell: (p: GridRenderCellParams<JoinedRow>) => {
          const ap = p.row.autopilot;
          if (!ap) {
            return (
              <CenteredCell>
                <Tooltip title="Autopilot has not reported this node yet (e.g. during post-failover cold-start).">
                  <Chip label="—" size="small" variant="outlined" />
                </Tooltip>
              </CenteredCell>
            );
          }
          return (
            <CenteredCell>
              <Tooltip title={`nodeStatus=${ap.nodeStatus}`}>
                <Chip
                  label={ap.healthy ? "healthy" : "unhealthy"}
                  size="small"
                  color={ap.healthy ? "success" : "error"}
                />
              </Tooltip>
            </CenteredCell>
          );
        },
      },
      {
        field: "lastContact",
        headerName: "Last Contact",
        width: 130,
        valueGetter: (_v: unknown, row: JoinedRow) =>
          row.isLeader ? -1 : (row.autopilot?.lastContactMs ?? -1),
        renderCell: (p: GridRenderCellParams<JoinedRow>) => {
          if (p.row.isLeader)
            return (
              <CenteredCell>
                <Typography variant="body2">—</Typography>
              </CenteredCell>
            );
          const ap = p.row.autopilot;
          if (!ap)
            return (
              <CenteredCell>
                <Typography variant="body2">—</Typography>
              </CenteredCell>
            );
          return (
            <CenteredCell>
              <Typography variant="body2">
                {formatLastContact(ap.lastContactMs)}
              </Typography>
            </CenteredCell>
          );
        },
      },
      {
        field: "lag",
        headerName: "Lag",
        width: 110,
        valueGetter: (_v: unknown, row: JoinedRow) => {
          if (row.isLeader || !row.autopilot) return 0;
          return Math.max(0, leaderIndex - row.autopilot.lastIndex);
        },
        renderCell: (p: GridRenderCellParams<JoinedRow>) => {
          if (p.row.isLeader)
            return (
              <CenteredCell>
                <Typography variant="body2">—</Typography>
              </CenteredCell>
            );
          const ap = p.row.autopilot;
          if (!ap)
            return (
              <CenteredCell>
                <Typography variant="body2">—</Typography>
              </CenteredCell>
            );
          const lag = Math.max(0, leaderIndex - ap.lastIndex);
          const MAX_TRAILING_LOGS = 500;
          const color = lag >= MAX_TRAILING_LOGS ? "warning" : "default";
          return (
            <CenteredCell>
              <Tooltip
                title={`Leader at index ${leaderIndex}; this peer at ${ap.lastIndex}`}
              >
                <Chip
                  label={`${lag}`}
                  size="small"
                  color={color}
                  variant="outlined"
                />
              </Tooltip>
            </CenteredCell>
          );
        },
      },
      {
        field: "actions",
        headerName: "Actions",
        type: "actions",
        width: 170,
        sortable: false,
        disableColumnMenu: true,
        getActions: (p: { row: JoinedRow }) => {
          const isSelf = p.row.nodeId === selfNodeId;
          const isCurrentLeader = p.row.nodeId === currentLeaderNodeId;
          const state = p.row.drainState;
          const canDrain = state === "active";
          const canResume = state !== "active";
          const canRemove = !isSelf && !isCurrentLeader && state === "drained";
          const canPromote = p.row.suffrage === "nonvoter";

          const promoteTitle = canPromote
            ? "Promote this non-voter to a full voting member."
            : "Already a voter.";

          const drainTitle = canDrain
            ? "Drain this node: transfer leadership if leader, notify RANs, stop BGP."
            : state === "draining"
              ? "Node is already draining; use Resume to reverse."
              : "Node is drained; use Resume to reverse or Remove to delete.";

          const resumeTitle = canResume
            ? "Resume: clear drain state and restart BGP. Does not reverse AMF Status Indication or reclaim leadership."
            : "Node is already active.";

          const removeTitle = isSelf
            ? "Cannot remove the node you are currently connected to."
            : isCurrentLeader
              ? "Cannot remove the current leader. Drain it first so leadership transfers, then retry."
              : state !== "drained"
                ? "Drain the node first. Remove is enabled only for nodes in the 'drained' state."
                : "Remove this node from the Raft cluster.";

          return [
            <Tooltip key="promote" title={promoteTitle}>
              <span>
                <GridActionsCellItem
                  icon={
                    <ArrowUpwardIcon
                      color={canPromote ? "primary" : "disabled"}
                    />
                  }
                  label="Promote to Voter"
                  disabled={!canPromote}
                  onClick={() => handlePromote(p.row)}
                />
              </span>
            </Tooltip>,
            <Tooltip key="drain" title={drainTitle}>
              <span>
                <GridActionsCellItem
                  icon={
                    <PowerSettingsNewIcon
                      color={canDrain ? "warning" : "disabled"}
                    />
                  }
                  label="Drain this node"
                  disabled={!canDrain}
                  onClick={() => setDrainTarget(p.row)}
                />
              </span>
            </Tooltip>,
            <Tooltip key="resume" title={resumeTitle}>
              <span>
                <GridActionsCellItem
                  icon={
                    <PlayArrowIcon color={canResume ? "success" : "disabled"} />
                  }
                  label="Resume this node"
                  disabled={!canResume}
                  onClick={() => setResumeTarget(p.row)}
                />
              </span>
            </Tooltip>,
            <Tooltip key="remove" title={removeTitle}>
              <span>
                <GridActionsCellItem
                  icon={
                    <DeleteIcon color={canRemove ? "primary" : "disabled"} />
                  }
                  label="Remove from Cluster"
                  disabled={!canRemove}
                  onClick={() => setRemoveTarget(p.row)}
                />
              </span>
            </Tooltip>,
          ];
        },
      } as GridColDef<JoinedRow>,
    ],
    [versionsDiffer, leaderIndex, selfNodeId, currentLeaderNodeId],
  );

  const statusLoaded = !statusQuery.isLoading;

  if (!statusLoaded) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
        <CircularProgress />
      </Box>
    );
  }

  if (!clusterEnabled) {
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
        <Typography variant="h4" sx={{ mb: 2 }}>
          Cluster
        </Typography>
        <Paper sx={{ p: 3 }}>
          <Typography variant="body1" sx={{ mb: 2 }}>
            This node is running in single-node mode. High availability is not
            enabled.
          </Typography>
          <Typography variant="body2" color="textSecondary">
            To enable HA, see the{" "}
            <a
              href="https://docs.ellanetworks.com"
              target="_blank"
              rel="noreferrer"
            >
              deployment guide
            </a>
            .
          </Typography>
        </Paper>
      </Box>
    );
  }

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <Grid container spacing={2} sx={{ mb: 2, alignItems: "center" }}>
        <Grid size={{ xs: 12, md: 8 }}>
          <Typography variant="h4">Cluster</Typography>
          <Typography variant="body1" color="textSecondary">
            Manage Raft cluster membership and observe live health.
          </Typography>
        </Grid>
        <Grid
          size={{ xs: 12, md: 4 }}
          sx={{
            display: "flex",
            justifyContent: { md: "flex-end" },
            gap: 1,
            flexWrap: "wrap",
          }}
        >
          <Button
            variant="contained"
            color="success"
            onClick={() => setMintOpen(true)}
          >
            Mint Join Token
          </Button>
          <Button
            variant="outlined"
            color="success"
            onClick={() => setAddOpen(true)}
          >
            Add Cluster Member
          </Button>
        </Grid>
      </Grid>

      <HealthBanner
        autopilot={autopilot}
        autopilotError={autopilotQuery.isError}
        versionsDiffer={versionsDiffer}
      />

      <ThemeProvider theme={gridTheme}>
        <DataGrid<JoinedRow>
          rows={rows}
          columns={columns}
          disableColumnMenu
          disableRowSelectionOnClick
          autoHeight
          hideFooter
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
          }}
        />
      </ThemeProvider>

      <ClusterPKIPanel pki={pkiQuery.data} error={pkiQuery.isError} />

      {isAddOpen && (
        <AddClusterMemberModal
          open
          onClose={() => setAddOpen(false)}
          onSuccess={() => {
            showSnackbar("Cluster member added.", "success");
            queryClient.invalidateQueries({ queryKey: ["cluster-members"] });
            queryClient.invalidateQueries({ queryKey: ["cluster-autopilot"] });
          }}
        />
      )}

      {isMintOpen && (
        <MintJoinTokenModal open onClose={() => setMintOpen(false)} />
      )}

      {drainTarget && (
        <DrainNodeModal
          open
          nodeId={drainTarget.nodeId}
          isLeader={drainTarget.isLeader}
          onClose={() => setDrainTarget(null)}
          onSuccess={handleDrainSuccess}
        />
      )}

      {resumeTarget && (
        <ResumeNodeModal
          open
          nodeId={resumeTarget.nodeId}
          onClose={() => setResumeTarget(null)}
          onSuccess={handleResumeSuccess}
        />
      )}

      {removeTarget && (
        <DeleteConfirmationModal
          open
          onClose={() => setRemoveTarget(null)}
          onConfirm={handleRemoveConfirm}
          title={`Remove node ${removeTarget.nodeId}?`}
          description={`Removes node ${removeTarget.nodeId} from the Raft cluster. The node must be shut down afterward — if it stays online, it will keep trying to rejoin.`}
        />
      )}
    </Box>
  );
};

export default ClusterPage;
