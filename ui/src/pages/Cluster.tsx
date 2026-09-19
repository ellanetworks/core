// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useCallback, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  Link as MuiLink,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import Grid from "@mui/material/Grid";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import OpenInNewIcon from "@mui/icons-material/OpenInNew";
import {
  type GridColDef,
  type GridRenderCellParams,
  GridActionsCellItem,
} from "@mui/x-data-grid";
import EntityGrid from "@/components/grid/EntityGrid";
import ArrowUpwardIcon from "@mui/icons-material/ArrowUpward";
import DeleteIcon from "@mui/icons-material/Delete";
import PowerSettingsNewIcon from "@mui/icons-material/PowerSettingsNew";
import PlayArrowIcon from "@mui/icons-material/PlayArrow";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { PRODUCT } from "@/utils/product";
import EmptyState from "@/components/EmptyState";
import { getStatus, type APIStatus } from "@/queries/status";
import {
  listClusterMembers,
  getAutopilotState,
  promoteClusterMember,
  type ClusterMember,
  type AutopilotServer,
  type AutopilotState,
  type DrainState,
  type DrainResponse,
} from "@/queries/cluster";
import AddNodeModal from "@/components/AddNodeModal";
import DrainNodeModal from "@/components/DrainNodeModal";
import ResumeNodeModal from "@/components/ResumeNodeModal";
import RemoveNodeModal, { nodeHealth } from "@/components/RemoveNodeModal";
import ClusterStateCard from "@/components/ClusterStateCard";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";
import { formatDateTime } from "@/utils/formatters";
import PageTitle from "@/components/PageTitle";

type JoinedRow = ClusterMember & {
  id: number;
  autopilot?: AutopilotServer;
};

const CLUSTER_PAGE_DESCRIPTION =
  "High-availability cluster members and health.";

const ActionSlot = React.forwardRef<
  HTMLSpanElement,
  React.ComponentPropsWithoutRef<"span"> & { touchRippleRef?: unknown }
>(function ActionSlot({ touchRippleRef, ...props }, ref) {
  void touchRippleRef;
  return <span ref={ref} {...props} />;
});

// MUI top-aligns renderCell output; vertical centering requires a full-height flex container.
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

function drainStateChip(state: DrainState, updatedAt?: string) {
  if (state === "draining") {
    const title = updatedAt
      ? `Draining since ${formatDateTime(updatedAt)}.`
      : "";
    return (
      <Tooltip title={title}>
        <Chip
          label="Draining"
          size="small"
          color="warning"
          variant="outlined"
        />
      </Tooltip>
    );
  }
  if (state === "drained") {
    const title = updatedAt ? `Drained at ${formatDateTime(updatedAt)}.` : "";
    return (
      <Tooltip title={title}>
        <Chip label="Drained" size="small" color="error" variant="outlined" />
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

const ClusterPage: React.FC = () => {
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const queryClient = useQueryClient();

  const [isMintOpen, setMintOpen] = useState(false);
  const [drainTarget, setDrainTarget] = useState<ClusterMember | null>(null);
  const [resumeTarget, setResumeTarget] = useState<ClusterMember | null>(null);
  const [removeTarget, setRemoveTarget] = useState<JoinedRow | null>(null);

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

  const members = useMemo(() => membersQuery.data ?? [], [membersQuery.data]);
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

  const versionsDiffer = useMemo(() => {
    const versions = new Set(
      members.map((m) => m.binaryVersion).filter((v) => v !== ""),
    );
    return versions.size > 1;
  }, [members]);

  const handlePromote = useCallback(
    async (m: ClusterMember) => {
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
    },
    [accessToken, showSnackbar, queryClient],
  );

  const handleRemoveSuccess = (nodeId: number) => {
    showSnackbar(`Node ${nodeId} removed.`, "success");
    queryClient.invalidateQueries({ queryKey: ["cluster-members"] });
    queryClient.invalidateQueries({ queryKey: ["cluster-autopilot"] });
  };

  const handleDrainSuccess = (result: DrainResponse) => {
    showSnackbar(`Drain ${result.drainState}.`, "success");
    queryClient.invalidateQueries({ queryKey: ["status"] });
    queryClient.invalidateQueries({ queryKey: ["cluster-members"] });
    queryClient.invalidateQueries({ queryKey: ["cluster-autopilot"] });
  };

  const handleResumeSuccess = () => {
    showSnackbar("Node resumed.", "success");
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
                <Tooltip title="The leader has not reported on this node yet.">
                  <Chip label="—" size="small" variant="outlined" />
                </Tooltip>
              </CenteredCell>
            );
          }
          return (
            <CenteredCell>
              <Chip
                label={ap.healthy ? "Healthy" : "Unhealthy"}
                size="small"
                color={ap.healthy ? "success" : "error"}
              />
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
          const isCurrentLeader = p.row.nodeId === currentLeaderNodeId;
          const state = p.row.drainState;
          const canDrain = state === "active";
          const canResume = state !== "active";
          const canRemove = !isCurrentLeader;
          const canPromote = p.row.suffrage === "nonvoter";

          const promoteTitle = canPromote
            ? "Promote this non-voter to a full voting member."
            : "Already a voter.";

          const drainTitle = canDrain
            ? "Drain this node."
            : "Node is drained; use Resume to reverse or Remove to delete.";

          const resumeTitle = canResume ? "Resume." : "Node is already active.";

          const removeTitle = isCurrentLeader
            ? "Cannot remove the current leader. Drain it first, then retry."
            : "Remove this node from the cluster.";

          return [
            <Tooltip key="promote" title={promoteTitle}>
              <ActionSlot>
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
              </ActionSlot>
            </Tooltip>,
            <Tooltip key="drain" title={drainTitle}>
              <ActionSlot>
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
              </ActionSlot>
            </Tooltip>,
            <Tooltip key="resume" title={resumeTitle}>
              <ActionSlot>
                <GridActionsCellItem
                  icon={
                    <PlayArrowIcon color={canResume ? "success" : "disabled"} />
                  }
                  label="Resume this node"
                  disabled={!canResume}
                  onClick={() => setResumeTarget(p.row)}
                />
              </ActionSlot>
            </Tooltip>,
            <Tooltip key="remove" title={removeTitle}>
              <ActionSlot>
                <GridActionsCellItem
                  icon={
                    <DeleteIcon color={canRemove ? "primary" : "disabled"} />
                  }
                  label="Remove from Cluster"
                  disabled={!canRemove}
                  onClick={() => setRemoveTarget(p.row)}
                />
              </ActionSlot>
            </Tooltip>,
          ];
        },
      } as GridColDef<JoinedRow>,
    ],
    [versionsDiffer, selfNodeId, currentLeaderNodeId, handlePromote],
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
        <PageTitle title="Cluster" />
        <Typography variant="body1" color="textSecondary">
          {CLUSTER_PAGE_DESCRIPTION}
        </Typography>

        <EmptyState
          primaryText="High availability is not enabled"
          secondaryText={
            <>
              This node is running in standalone mode.{" "}
              <MuiLink
                href={PRODUCT.haDocsUrl}
                target="_blank"
                rel="noreferrer"
                underline="hover"
                sx={{ display: "inline-flex", alignItems: "center" }}
              >
                Learn more
                <OpenInNewIcon sx={{ fontSize: 16, ml: 0.5 }} />
              </MuiLink>
            </>
          }
        />
      </Box>
    );
  }

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <Grid container spacing={2} sx={{ mb: 2, alignItems: "center" }}>
        <Grid size={{ xs: 12, md: 8 }}>
          <PageTitle title="Cluster" />
          <Typography variant="body1" color="textSecondary">
            {CLUSTER_PAGE_DESCRIPTION}
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
            Add Node
          </Button>
        </Grid>
      </Grid>

      <ClusterStateCard status={statusQuery.data} autopilot={autopilot} />

      <Typography variant="h6" sx={{ mb: 1.5 }}>
        Nodes
      </Typography>

      <EntityGrid<JoinedRow>
        rows={rows}
        columns={columns}
        hideFooter
        defaultPageSize={100}
      />

      {isMintOpen && <AddNodeModal open onClose={() => setMintOpen(false)} />}

      {drainTarget && (
        <DrainNodeModal
          open
          nodeId={drainTarget.nodeId}
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
        <RemoveNodeModal
          open
          nodeId={removeTarget.nodeId}
          drainState={removeTarget.drainState}
          health={nodeHealth(removeTarget.autopilot)}
          isSelf={removeTarget.nodeId === selfNodeId}
          onClose={() => setRemoveTarget(null)}
          onSuccess={() => handleRemoveSuccess(removeTarget.nodeId)}
        />
      )}
    </Box>
  );
};

export default ClusterPage;
