import React, { useMemo } from "react";
import {
  Card,
  CardActionArea,
  CardContent,
  CardHeader,
  Chip,
  Stack,
  Typography,
  Tooltip,
} from "@mui/material";
import HubIcon from "@mui/icons-material/Hub";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { getStatus, type APIStatus } from "@/queries/status";
import {
  listClusterMembers,
  getAutopilotState,
  type ClusterMember,
  type AutopilotState,
} from "@/queries/cluster";

interface Props {
  showSnackbar?: ReturnType<typeof useSnackbar>["showSnackbar"];
}

// ClusterSummaryCard renders a compact HA overview for the dashboard.
// Visible to all authenticated roles; admins get a clickable card that
// navigates to /cluster.
const ClusterSummaryCard: React.FC<Props> = () => {
  const { accessToken, authReady, role } = useAuth();
  const isAdmin = role === "Admin";

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
  });

  const members = membersQuery.data ?? [];
  const autopilot = autopilotQuery.data;

  const { voters, nonvoters } = useMemo(() => {
    let v = 0;
    let nv = 0;
    for (const m of members) {
      if (m.suffrage === "voter") v += 1;
      else if (m.suffrage === "nonvoter") nv += 1;
    }
    return { voters: v, nonvoters: nv };
  }, [members]);

  const versionsDiffer = useMemo(() => {
    const set = new Set(
      members.map((m) => m.binaryVersion).filter((v) => v !== ""),
    );
    return set.size > 1;
  }, [members]);

  if (!clusterEnabled) return null;

  const cluster = statusQuery.data?.cluster;
  const leaderLabel =
    cluster && cluster.leaderNodeId
      ? `node ${cluster.leaderNodeId}${cluster.leaderAPIAddress ? ` (${cluster.leaderAPIAddress})` : ""}`
      : "unknown";

  const ft = autopilot?.failureTolerance;
  const healthSeverity: "success" | "warning" | "error" = !autopilot
    ? "warning"
    : !autopilot.healthy || (ft !== undefined && ft < 0)
      ? "error"
      : ft === 0
        ? "warning"
        : "success";

  const healthLabel = !autopilot
    ? "unknown"
    : autopilot.healthy
      ? "healthy"
      : "unhealthy";

  const content = (
    <CardContent>
      <Stack spacing={1.5}>
        <Stack
          direction="row"
          spacing={1}
          sx={{ alignItems: "center", flexWrap: "wrap" }}
        >
          <Chip
            label={healthLabel}
            size="small"
            color={healthSeverity}
            variant="filled"
          />
          {autopilot && (
            <Tooltip title="Voter failures the cluster can absorb before writes stall.">
              <Chip
                label={`Failure tolerance: ${ft}`}
                size="small"
                variant="outlined"
                color={ft === 0 ? "warning" : "default"}
              />
            </Tooltip>
          )}
          {versionsDiffer && (
            <Chip
              label="version skew"
              size="small"
              color="warning"
              variant="outlined"
            />
          )}
        </Stack>
        <Typography variant="body2">
          <strong>Leader:</strong> {leaderLabel}
        </Typography>
        <Typography variant="body2">
          <strong>Members:</strong> {voters} voter{voters === 1 ? "" : "s"},{" "}
          {nonvoters} non-voter{nonvoters === 1 ? "" : "s"}
        </Typography>
      </Stack>
    </CardContent>
  );

  return (
    <Card variant="outlined">
      <CardHeader
        avatar={<HubIcon color="primary" />}
        title="Cluster"
        subheader={
          cluster && (
            <>
              This node is <strong>{cluster.role}</strong> (id {cluster.nodeId})
            </>
          )
        }
      />
      {isAdmin ? (
        <CardActionArea component={Link} to="/cluster">
          {content}
        </CardActionArea>
      ) : (
        content
      )}
    </Card>
  );
};

export default ClusterSummaryCard;
