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
import { useSnackbar } from "@/contexts/SnackbarContext";
import type { APIStatus } from "@/queries/status";
import type { AutopilotState } from "@/queries/cluster";

const CONVERGING =
  "The leader has not reported yet. This is normal for a moment after a leadership change.";

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

const CopyableValue: React.FC<{ value: string }> = ({ value }) => {
  const { showSnackbar } = useSnackbar();

  const handleCopy = async () => {
    if (!navigator.clipboard) {
      showSnackbar(
        "Clipboard API not available. Please use HTTPS or try a different browser.",
        "error",
      );
      return;
    }
    try {
      await navigator.clipboard.writeText(value);
      showSnackbar("Copied to clipboard.", "success");
    } catch {
      showSnackbar("Failed to copy.", "error");
    }
  };

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
        <IconButton size="small" onClick={handleCopy} aria-label="Copy">
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
      <Tooltip title="At the quorum limit. One more voter failure would stop writes.">
        <Chip
          label="No failure tolerance"
          size="small"
          color="warning"
          variant="outlined"
        />
      </Tooltip>
    );
  }

  const plural = failureTolerance === 1 ? "" : "s";

  return (
    <Tooltip
      title={`The cluster keeps accepting writes while up to ${failureTolerance} voter${plural} are down.`}
    >
      <Chip
        label={`Tolerates ${failureTolerance} voter failure${plural}`}
        size="small"
        color="success"
        variant="outlined"
      />
    </Tooltip>
  );
};

const HealthValue: React.FC<{
  autopilot?: AutopilotState;
  hasLeader: boolean;
}> = ({ autopilot, hasLeader }) => {
  if (!hasLeader) {
    return (
      <Tooltip title="No node holds leadership. The cluster cannot accept writes until enough voters recover for an election to succeed.">
        <Chip label="No leader" size="small" color="error" />
      </Tooltip>
    );
  }

  if (!autopilot) {
    return <UnknownChip title={CONVERGING} />;
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

const SchemaValue: React.FC<{ status?: APIStatus }> = ({ status }) => {
  const applied = status?.cluster?.appliedSchemaVersion;
  const binary = status?.schemaVersion;
  const pending = status?.cluster?.pendingMigration;

  if (applied === undefined) {
    return (
      <UnknownChip title="The committed schema version is not available." />
    );
  }

  if (!pending) {
    return (
      <Tooltip title="Every node has committed this schema version.">
        <Typography variant="body2">{`v${applied}`}</Typography>
      </Tooltip>
    );
  }

  const supports =
    binary === undefined ? "" : ` This node's binary supports v${binary}.`;

  if (pending.laggardNodeId) {
    return (
      <Tooltip
        title={`The cluster has committed schema v${pending.currentSchema} and cannot advance.${supports} Upgrade or remove node ${pending.laggardNodeId} to let the migration proceed.`}
      >
        <Chip
          label={`v${pending.currentSchema} · blocked by node ${pending.laggardNodeId}`}
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
}

const ClusterStateCard: React.FC<ClusterStateCardProps> = ({
  status,
  autopilot,
}) => {
  const cluster = status?.cluster;
  const hasLeader = (cluster?.leaderNodeId ?? 0) !== 0;

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
              <CopyableValue value={cluster.clusterId} />
            ) : undefined
          }
        />
        <InfoRow
          label="Health"
          value={<HealthValue autopilot={autopilot} hasLeader={hasLeader} />}
        />
        <InfoRow label="Schema" value={<SchemaValue status={status} />} />
      </CardContent>
    </Card>
  );
};

export default ClusterStateCard;
