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

const HealthValue: React.FC<{ autopilot?: AutopilotState }> = ({
  autopilot,
}) => {
  if (!autopilot) {
    return (
      <UnknownChip title="Autopilot has not reported yet. It typically takes a moment to converge after a leadership change." />
    );
  }
  return (
    <Chip
      label={autopilot.healthy ? "Healthy" : "Unhealthy"}
      size="small"
      color={autopilot.healthy ? "success" : "error"}
    />
  );
};

const FailureToleranceValue: React.FC<{ autopilot?: AutopilotState }> = ({
  autopilot,
}) => {
  if (!autopilot) {
    return (
      <UnknownChip title="Autopilot has not reported yet. It typically takes a moment to converge after a leadership change." />
    );
  }

  const ft = autopilot.failureTolerance;

  if (ft < 0) {
    return (
      <Tooltip title="Quorum is lost. The cluster cannot accept writes until a voter recovers.">
        <Chip label="Quorum lost" size="small" color="error" />
      </Tooltip>
    );
  }

  if (ft === 0) {
    return (
      <Tooltip title="At the quorum limit. One more voter failure would stop writes.">
        <Chip
          label="0 voters"
          size="small"
          color="warning"
          variant="outlined"
        />
      </Tooltip>
    );
  }

  return (
    <Tooltip
      title={`The cluster keeps accepting writes while up to ${ft} voter${ft === 1 ? "" : "s"} are down.`}
    >
      <Chip
        label={`${ft} voter${ft === 1 ? "" : "s"}`}
        size="small"
        color="success"
        variant="outlined"
      />
    </Tooltip>
  );
};

const SchemaValue: React.FC<{
  applied?: number;
  binary?: number;
}> = ({ applied, binary }) => {
  if (applied === undefined && binary === undefined) {
    return <UnknownChip title="Schema versions are not available yet." />;
  }

  const appliedText = applied === undefined ? "unknown" : `v${applied}`;

  if (binary === undefined || applied === undefined || applied === binary) {
    return <Typography variant="body2">{appliedText}</Typography>;
  }

  return (
    <Box
      sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}
    >
      <Typography variant="body2">{appliedText}</Typography>
      <Tooltip
        title={`The cluster has committed schema v${applied}; this node's binary supports v${binary}. They differ only during a rolling upgrade.`}
      >
        <Chip
          label={`this node supports v${binary}`}
          size="small"
          color="warning"
          variant="outlined"
        />
      </Tooltip>
    </Box>
  );
};

const VersionsValue: React.FC<{ versions: string[] }> = ({ versions }) => {
  if (versions.length === 1) {
    return <Typography variant="body2">{versions[0]}</Typography>;
  }

  return (
    <Tooltip title="Nodes are running different binary versions. This is expected during a rolling upgrade but should not persist.">
      <Chip
        label={`Mixed — ${versions.join(", ")}`}
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
  versions: string[];
}

const ClusterStateCard: React.FC<ClusterStateCardProps> = ({
  status,
  autopilot,
  versions,
}) => {
  const cluster = status?.cluster;
  const leaderNodeId = cluster?.leaderNodeId || autopilot?.leaderNodeId || 0;
  const voters = autopilot?.voters;
  const pending = cluster?.pendingMigration;

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
        <InfoRow label="Health" value={<HealthValue autopilot={autopilot} />} />
        <InfoRow
          label="Failure tolerance"
          value={<FailureToleranceValue autopilot={autopilot} />}
        />
        <InfoRow
          label="Voters"
          value={
            voters === undefined ? (
              <UnknownChip title="Autopilot has not reported yet. It typically takes a moment to converge after a leadership change." />
            ) : (
              `${voters.length} (${voters.length === 0 ? "none" : voters.map((id) => `node ${id}`).join(", ")})`
            )
          }
        />
        <InfoRow
          label="Leader"
          value={leaderNodeId ? `Node ${leaderNodeId}` : "No leader"}
        />
        <InfoRow
          label="Schema"
          value={
            <SchemaValue
              applied={cluster?.appliedSchemaVersion}
              binary={status?.schemaVersion}
            />
          }
        />
        <InfoRow
          label="Pending migration"
          value={
            pending ? (
              <Tooltip
                title={
                  pending.laggardNodeId
                    ? `Node ${pending.laggardNodeId} is still on an older binary and is blocking the migration. Upgrade or remove it to let the cluster advance.`
                    : "A schema migration is pending cluster-wide."
                }
              >
                <Chip
                  label={
                    pending.laggardNodeId
                      ? `v${pending.currentSchema} → v${pending.targetSchema}, blocked by node ${pending.laggardNodeId}`
                      : `v${pending.currentSchema} → v${pending.targetSchema}`
                  }
                  size="small"
                  color="warning"
                  variant="outlined"
                />
              </Tooltip>
            ) : (
              "None"
            )
          }
        />
        <InfoRow
          label="Binary versions"
          value={
            versions.length === 0 ? undefined : (
              <VersionsValue versions={versions} />
            )
          }
        />
      </CardContent>
    </Card>
  );
};

export default ClusterStateCard;
