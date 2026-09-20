// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { NodeId } from "@/queries/nodeId";
import { Box, Checkbox, FormControlLabel, Typography } from "@mui/material";
import {
  removeClusterMember,
  type DrainState,
  type AutopilotServer,
} from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";
import ConfirmDialog from "@/components/form/ConfirmDialog";

export type NodeHealth = "healthy" | "unhealthy" | "unknown";

export function nodeHealth(autopilot?: AutopilotServer): NodeHealth {
  if (!autopilot) return "unknown";
  return autopilot.healthy ? "healthy" : "unhealthy";
}

export function defaultForce(health: NodeHealth): boolean {
  return health === "unhealthy";
}

interface Props {
  open: boolean;
  nodeId: NodeId;
  nodeLabel: string;
  drainState: DrainState;
  health: NodeHealth;
  isSelf: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const RemoveNodeModal: React.FC<Props> = ({
  open,
  nodeId,
  nodeLabel,
  drainState,
  health,
  isSelf,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();
  const forceRequired = drainState !== "drained";
  const [force, setForce] = useState(forceRequired && defaultForce(health));

  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setForce(forceRequired && defaultForce(health));
  }

  const handleConfirm = async () => {
    if (!accessToken) return;
    await removeClusterMember(accessToken, nodeId, force);
    onSuccess();
    onClose();
  };

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={handleConfirm}
      title={`Remove node ${nodeLabel}?`}
      description={
        <>
          <strong>Node {nodeLabel}</strong> will keep trying to rejoin unless
          you shut it down afterward.
        </>
      }
      extra={
        <Box sx={{ mt: 2 }}>
          {forceRequired && (
            <FormControlLabel
              control={
                <Checkbox
                  checked={force}
                  onChange={(e) => setForce(e.target.checked)}
                />
              }
              label="Force remove (skip drain)"
            />
          )}

          {isSelf && (
            <Typography variant="body2" color="warning.main" sx={{ mt: 1 }}>
              This is the node serving this page. Removing it ends your session.
            </Typography>
          )}
        </Box>
      }
      confirmLabel="Remove"
      confirmingLabel="Removing…"
      confirmColor="error"
      confirmDisabled={forceRequired && !force}
      fullWidth
    />
  );
};

export default RemoveNodeModal;
