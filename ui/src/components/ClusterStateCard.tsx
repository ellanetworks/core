// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import {
  Box,
  Card,
  CardContent,
  Chip,
  IconButton,
  Tooltip,
  Typography,
} from "@mui/material";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import { useCopyToClipboard } from "@/hooks/useCopyToClipboard";
import type { APIStatus } from "@/queries/status";
import type { AutopilotState, ClusterMember } from "@/queries/cluster";
import { nodeIdKey, nodeLabel, sameNodeId } from "@/queries/nodeId";

const CONVERGING =
  "The leader has not reported yet. This is normal for a moment after a leadership change.";

const errorMessage = (err: unknown): string =>
  err instanceof Error ? err.message : "unknown error";

const InfoRow: React.FC<{
  label: string;
  value?: React.ReactNode;
}> = ({ label, value }) => {
  const isEmpty = value === undefined || value === "" || value === null;
  const display = isEmpty ? "—" : value;

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        py: 0.75,
        minHeight: 40,
        "&:not(:last-child)": {
          borderBottom: "1px solid",
          borderColor: "divider",
        },
      }}
    >
      <Typography
        variant="body2"
        sx={{ color: "text.secondary", minWidth: 180, flexShrink: 0, mr: 2 }}
      >
        {label}
      </Typography>
      {typeof display === "string" || typeof display === "number" ? (
        <Typography variant="body2">{display}</Typography>
      ) : (
        display
      )}
    </Box>
  );
};

const UnknownChip: React.FC<{ title: string }> = ({ title }) => (
  <Tooltip title={title}>
    <Chip label="Unknown" size="small" variant="outlined" />
  </Tooltip>
);

const CopyableValue: React.FC<{ value: string; label: string }> = ({
  value,
  label,
}) => {
  const copy = useCopyToClipboard();

  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, minWidth: 0 }}>
      <Typography
        variant="body2"
        sx={{ overflow: "hidden", textOverflow: "ellipsis" }}
        title={value}
      >
        {value}
      </Typography>
      <Tooltip title="Copy">
        <IconButton
          size="small"
          onClick={() => copy(value, label)}
          aria-label="Copy"
        >
          <ContentCopyIcon fontSize="inherit" />
        </IconButton>
      </Tooltip>
    </Box>
  );
};

const ToleranceChip: React.FC<{ failureTolerance: number }> = ({
  failureTolerance,
}) => {
  if (failureTolerance < 1) {
    return (
      <Chip
        label="No failure tolerance"
        size="small"
        color="warning"
        variant="outlined"
      />
    );
  }

  const plural = failureTolerance === 1 ? "" : "s";

  return (
    <Chip
      label={`Tolerates ${failureTolerance} voter failure${plural}`}
      size="small"
      color="success"
      variant="outlined"
    />
  );
};

const HealthValue: React.FC<{
  autopilot?: AutopilotState;
  autopilotError?: unknown;
  hasLeader: boolean;
}> = ({ autopilot, autopilotError, hasLeader }) => {
  if (!hasLeader) {
    return (
      <Tooltip title="No node holds leadership. The cluster cannot accept writes until enough voters recover for an election to succeed.">
        <Chip label="No leader" size="small" color="error" />
      </Tooltip>
    );
  }

  if (!autopilot) {
    return (
      <UnknownChip
        title={
          autopilotError
            ? `Health could not be read: ${errorMessage(autopilotError)}`
            : CONVERGING
        }
      />
    );
  }

  return (
    <Box
      sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}
    >
      <Chip
        label={autopilot.healthy ? "Healthy" : "Unhealthy"}
        size="small"
        color={autopilot.healthy ? "success" : "error"}
      />
      <ToleranceChip failureTolerance={autopilot.failureTolerance} />
    </Box>
  );
};

const SchemaValue: React.FC<{
  status?: APIStatus;
  members: ClusterMember[];
}> = ({ status, members }) => {
  const applied = status?.cluster?.appliedSchemaVersion;
  const binary = status?.schemaVersion;
  const pending = status?.cluster?.pendingMigration;

  if (applied === undefined) {
    return (
      <UnknownChip title="The committed schema version is not available." />
    );
  }

  if (!pending) {
    return <Typography variant="body2">{`v${applied}`}</Typography>;
  }

  const supports =
    binary === undefined ? "" : ` This node's binary supports v${binary}.`;

  if (pending.laggardNodeId) {
    const laggardId = pending.laggardNodeId;
    const laggard = members.find((m) => sameNodeId(m.nodeId, laggardId));
    const laggardLabel = nodeLabel(laggard?.displayName, laggardId);

    return (
      <Tooltip
        title={`The cluster has committed schema v${pending.currentSchema} and cannot advance.${supports} Upgrade or remove node ${laggardLabel} to let the migration proceed.`}
      >
        <Chip
          label={`v${pending.currentSchema} · blocked by node ${laggardLabel}`}
          size="small"
          color="warning"
          variant="outlined"
        />
      </Tooltip>
    );
  }

  return (
    <Tooltip
      title={`The cluster has committed schema v${pending.currentSchema} and is migrating to v${pending.targetSchema}.${supports}`}
    >
      <Chip
        label={`v${pending.currentSchema} → v${pending.targetSchema}`}
        size="small"
        color="warning"
        variant="outlined"
      />
    </Tooltip>
  );
};

interface ClusterStateCardProps {
  status?: APIStatus;
  autopilot?: AutopilotState;
  autopilotError?: unknown;
  members?: ClusterMember[];
}

const ClusterStateCard: React.FC<ClusterStateCardProps> = ({
  status,
  autopilot,
  autopilotError,
  members = [],
}) => {
  const cluster = status?.cluster;
  const hasLeader = !["", "0"].includes(nodeIdKey(cluster?.leaderNodeId));

  return (
    <Card variant="outlined" sx={{ mb: 2 }}>
      <CardContent>
        <Typography variant="h6" sx={{ mb: 1.5 }}>
          State
        </Typography>
        <InfoRow
          label="Cluster ID"
          value={
            cluster?.clusterId ? (
              <CopyableValue value={cluster.clusterId} label="Cluster ID" />
            ) : undefined
          }
        />
        <InfoRow
          label="Health"
          value={
            <HealthValue
              autopilot={autopilot}
              autopilotError={autopilotError}
              hasLeader={hasLeader}
            />
          }
        />
        <InfoRow
          label="Schema"
          value={<SchemaValue status={status} members={members} />}
        />
      </CardContent>
    </Card>
  );
};

export default ClusterStateCard;
