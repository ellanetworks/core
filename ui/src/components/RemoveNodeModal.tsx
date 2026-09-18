// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import {
  Alert,
  Box,
  Checkbox,
  FormControlLabel,
  Typography,
} from "@mui/material";
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

const FORCE_HELP: Record<NodeHealth, string> = {
  unhealthy:
    "Autopilot reports this node as unhealthy, so it is unlikely to ever reach the drained state on its own.",
  healthy:
    "Autopilot reports this node as healthy. Drain it first unless you have a reason not to.",
  unknown:
    "Autopilot has not reported this node yet. That happens to every node for a moment after a leadership change, so it does not on its own mean the node is down.",
};

interface Props {
  open: boolean;
  nodeId: number;
  drainState: DrainState;
  health: NodeHealth;
  isSelf: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const RemoveNodeModal: React.FC<Props> = ({
  open,
  nodeId,
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
      title={`Remove node ${nodeId}?`}
      description={
        <>
          Removes <strong>node {nodeId}</strong> from the Raft cluster. Shut the
          node down afterward — if it stays online, it will keep trying to
          rejoin.
        </>
      }
      extra={
        <Box sx={{ mt: 2 }}>
          {forceRequired && (
            <>
              <FormControlLabel
                control={
                  <Checkbox
                    checked={force}
                    onChange={(e) => setForce(e.target.checked)}
                  />
                }
                label="Force remove (skip drain)"
              />
              <Typography
                variant="body2"
                color="textSecondary"
                sx={{ mb: 1.5 }}
              >
                This node is <em>{drainState}</em>, not <em>drained</em>, so the
                cluster will refuse the removal unless you skip the drain.{" "}
                {FORCE_HELP[health]}
              </Typography>
            </>
          )}

          {force && (
            <Alert severity="warning" sx={{ mb: isSelf ? 1.5 : 0 }}>
              Skipping drain drops this node&apos;s work instead of migrating
              it. Connected radios are not told to reselect, BGP routes are
              withdrawn without warning, and the 4G and 5G subscribers on this
              node lose their sessions and must re-attach.
            </Alert>
          )}

          {isSelf && (
            <Alert severity="warning">
              This is the node serving this page. Removing it ends your session
              here — reconnect to another node to keep managing the cluster.
            </Alert>
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
