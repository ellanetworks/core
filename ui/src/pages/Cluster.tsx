// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useCallback, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Link as MuiLink,
  ListItemText,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import Grid from "@mui/material/Grid";
import OpenInNewIcon from "@mui/icons-material/OpenInNew";
import {
  type GridColDef,
  type GridRenderCellParams,
  GridActionsCellItem,
} from "@mui/x-data-grid";
import EntityGrid from "@/components/grid/EntityGrid";
import { useAuth } from "@/contexts/AuthContext";
import { useStatusQuery } from "@/hooks/useStatus";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { PRODUCT } from "@/utils/product";
import EmptyState from "@/components/EmptyState";
import QueryState from "@/components/QueryState";
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
import RenameNodeModal from "@/components/RenameNodeModal";
import {
  nodeIdKey,
  nodeLabel,
  sameNodeId,
  shortNodeId,
} from "@/queries/nodeId";

type JoinedRow = ClusterMember & {
  id: string;
  autopilot?: AutopilotServer;
};

const CLUSTER_PAGE_DESCRIPTION =
  "High-availability cluster members and health.";

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

const TruncatedText: React.FC<{ value: string }> = ({ value }) => (
  <CenteredCell>
    <Typography variant="body2" noWrap title={value || undefined}>
      {value || "—"}
    </Typography>
  </CenteredCell>
);

const ClusterPage: React.FC = () => {
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const queryClient = useQueryClient();

  const [isMintOpen, setMintOpen] = useState(false);
  const [drainTarget, setDrainTarget] = useState<ClusterMember | null>(null);
  const [resumeTarget, setResumeTarget] = useState<ClusterMember | null>(null);
  const [removeTarget, setRemoveTarget] = useState<JoinedRow | null>(null);
  const [renameTarget, setRenameTarget] = useState<JoinedRow | null>(null);

  const statusQuery = useStatusQuery();

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
  const autopilotError = autopilotQuery.isError
    ? autopilotQuery.error
    : undefined;
  const autopilot = autopilotError ? undefined : autopilotQuery.data;

  const rows: JoinedRow[] = useMemo(() => {
    const apByNode = new Map<string, AutopilotServer>();
    for (const s of autopilot?.servers ?? []) {
      apByNode.set(nodeIdKey(s.nodeId), s);
    }
    return members.map((m) => ({
      ...m,
      id: nodeIdKey(m.nodeId),
      autopilot: apByNode.get(nodeIdKey(m.nodeId)),
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
        showSnackbar(
          `Node ${nodeLabel(m.displayName, m.nodeId)} promoted to voter.`,
          "success",
        );
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

  const handleRemoveSuccess = (member: JoinedRow) => {
    const id = nodeIdKey(member.nodeId);
    showSnackbar(
      member.displayName
        ? `Node ${member.displayName} (${id}) removed.`
        : `Node ${id} removed.`,
      "success",
    );
    queryClient.invalidateQueries({ queryKey: ["cluster-members"] });
    queryClient.invalidateQueries({ queryKey: ["cluster-autopilot"] });
  };

  const handleDrainSuccess = (member: ClusterMember, result: DrainResponse) => {
    const label = nodeLabel(member.displayName, member.nodeId);
    showSnackbar(
      result.drainState === "drained"
        ? `Node ${label} drained.`
        : `Node ${label} is draining. Subscribers are being moved.`,
      "success",
    );
    queryClient.invalidateQueries({ queryKey: ["status"] });
    queryClient.invalidateQueries({ queryKey: ["cluster-members"] });
    queryClient.invalidateQueries({ queryKey: ["cluster-autopilot"] });
  };

  const handleResumeSuccess = () => {
    showSnackbar("Node resumed.", "success");
    queryClient.invalidateQueries({ queryKey: ["cluster-members"] });
    queryClient.invalidateQueries({ queryKey: ["cluster-autopilot"] });
  };

  const selfNodeId = statusQuery.data?.cluster?.nodeId;
  const currentLeaderNodeId = statusQuery.data?.cluster?.leaderNodeId;

  const columns: GridColDef<JoinedRow>[] = useMemo(
    () => [
      {
        field: "nodeId",
        headerName: "Node",
        flex: 1.5,
        minWidth: 260,
        renderCell: (p: GridRenderCellParams<JoinedRow>) => {
          const isLeader = sameNodeId(p.row.nodeId, currentLeaderNodeId);
          return (
            <Stack
              direction="row"
              spacing={1}
              sx={{ alignItems: "center", height: "100%", minWidth: 0 }}
            >
              <Typography
                variant="body2"
                noWrap
                title={
                  p.row.displayName
                    ? `${p.row.displayName} (${nodeIdKey(p.row.nodeId)})`
                    : nodeIdKey(p.row.nodeId)
                }
                sx={{ fontWeight: isLeader ? 700 : 400, minWidth: 0 }}
              >
                {nodeLabel(p.row.displayName, p.row.nodeId)}
                {p.row.displayName !== "" && (
                  <Typography
                    component="span"
                    variant="caption"
                    sx={{
                      ml: 1,
                      color: "text.secondary",
                      fontFamily: "monospace",
                      fontWeight: 400,
                    }}
                  >
                    {shortNodeId(p.row.nodeId)}
                  </Typography>
                )}
              </Typography>
              {isLeader && (
                <Chip
                  label="Leader"
                  color="primary"
                  size="small"
                  sx={{ flexShrink: 0 }}
                />
              )}
              {sameNodeId(p.row.nodeId, selfNodeId) && (
                <Chip
                  label="This node"
                  size="small"
                  variant="outlined"
                  sx={{ flexShrink: 0 }}
                />
              )}
            </Stack>
          );
        },
      },
      {
        field: "raftAddress",
        headerName: "Cluster Address",
        flex: 1,
        minWidth: 180,
        renderCell: (p: GridRenderCellParams<JoinedRow>) => (
          <TruncatedText value={p.row.raftAddress} />
        ),
      },
      {
        field: "apiAddress",
        headerName: "API Address",
        flex: 1,
        minWidth: 190,
        renderCell: (p: GridRenderCellParams<JoinedRow>) => (
          <TruncatedText value={p.row.apiAddress} />
        ),
      },
      {
        field: "suffrage",
        headerName: "Suffrage",
        width: 100,
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
        width: 100,
        renderCell: (p: GridRenderCellParams<JoinedRow>) => (
          <CenteredCell>
            {drainStateChip(p.row.drainState, p.row.drainUpdatedAt)}
          </CenteredCell>
        ),
      },
      {
        field: "binaryVersion",
        headerName: "Version",
        width: 100,
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
        width: 100,
        renderCell: (p: GridRenderCellParams<JoinedRow>) => {
          const ap = p.row.autopilot;
          if (!ap) {
            return (
              <CenteredCell>
                <Tooltip
                  title={
                    autopilotError
                      ? `Health could not be read: ${autopilotError instanceof Error ? autopilotError.message : "unknown error"}`
                      : "The leader has not reported on this node yet."
                  }
                >
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
        width: 90,
        sortable: false,
        disableColumnMenu: true,
        getActions: (p: { row: JoinedRow }) => {
          const isCurrentLeader = sameNodeId(p.row.nodeId, currentLeaderNodeId);
          const state = p.row.drainState;

          const actions = [
            <GridActionsCellItem
              key="rename"
              showInMenu
              label="Rename this node"
              onClick={() => setRenameTarget(p.row)}
            />,
          ];

          if (p.row.suffrage === "nonvoter") {
            actions.push(
              <GridActionsCellItem
                key="promote"
                showInMenu
                label="Promote to Voter"
                onClick={() => handlePromote(p.row)}
              />,
            );
          }

          actions.push(
            state === "active" ? (
              <GridActionsCellItem
                key="drain"
                showInMenu
                label="Drain this node"
                onClick={() => setDrainTarget(p.row)}
              />
            ) : (
              <GridActionsCellItem
                key="resume"
                showInMenu
                label="Resume this node"
                onClick={() => setResumeTarget(p.row)}
              />
            ),
            <GridActionsCellItem
              key="remove"
              showInMenu
              label={
                isCurrentLeader ? (
                  <ListItemText
                    primary="Remove from Cluster"
                    secondary="Drain the leader first."
                  />
                ) : (
                  "Remove from Cluster"
                )
              }
              disabled={isCurrentLeader}
              onClick={() => setRemoveTarget(p.row)}
            />,
          );

          return actions;
        },
      } as GridColDef<JoinedRow>,
    ],
    [
      versionsDiffer,
      selfNodeId,
      currentLeaderNodeId,
      handlePromote,
      autopilotError,
    ],
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
              This node is running in standalone mode. Set the cluster bind
              address in the config file and restart this node to enable
              clustering.{" "}
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

      <ClusterStateCard
        status={statusQuery.data}
        autopilot={autopilot}
        autopilotError={autopilotError}
        members={members}
      />

      <Typography variant="h6" sx={{ mb: 1.5 }}>
        Nodes
      </Typography>

      <QueryState query={membersQuery} resource="cluster members">
        {() => (
          <EntityGrid<JoinedRow>
            rows={rows}
            columns={columns}
            hideFooter
            defaultPageSize={100}
          />
        )}
      </QueryState>

      {isMintOpen && (
        <AddNodeModal
          open
          clusterAddress={
            members.find((m) => sameNodeId(m.nodeId, selfNodeId))?.raftAddress
          }
          onClose={() => setMintOpen(false)}
        />
      )}

      {renameTarget && (
        <RenameNodeModal
          open
          nodeId={renameTarget.nodeId}
          initialDisplayName={renameTarget.displayName}
          onClose={() => setRenameTarget(null)}
          onSuccess={() => {
            showSnackbar("Display name updated.", "success");
            queryClient.invalidateQueries({ queryKey: ["cluster-members"] });
          }}
        />
      )}

      {drainTarget && (
        <DrainNodeModal
          open
          nodeId={drainTarget.nodeId}
          nodeLabel={nodeLabel(drainTarget.displayName, drainTarget.nodeId)}
          isLastActive={
            !members.some(
              (m) =>
                !sameNodeId(m.nodeId, drainTarget.nodeId) &&
                m.drainState === "active",
            )
          }
          isLeader={sameNodeId(drainTarget.nodeId, currentLeaderNodeId)}
          isSelf={sameNodeId(drainTarget.nodeId, selfNodeId)}
          onClose={() => setDrainTarget(null)}
          onSuccess={(result) => handleDrainSuccess(drainTarget, result)}
        />
      )}

      {resumeTarget && (
        <ResumeNodeModal
          open
          nodeId={resumeTarget.nodeId}
          nodeLabel={nodeLabel(resumeTarget.displayName, resumeTarget.nodeId)}
          onClose={() => setResumeTarget(null)}
          onSuccess={handleResumeSuccess}
        />
      )}

      {removeTarget && (
        <RemoveNodeModal
          open
          nodeId={removeTarget.nodeId}
          nodeLabel={nodeLabel(removeTarget.displayName, removeTarget.nodeId)}
          drainState={removeTarget.drainState}
          health={nodeHealth(removeTarget.autopilot)}
          isSelf={sameNodeId(removeTarget.nodeId, selfNodeId)}
          onClose={() => setRemoveTarget(null)}
          onSuccess={() => handleRemoveSuccess(removeTarget)}
        />
      )}
    </Box>
  );
};

export default ClusterPage;
