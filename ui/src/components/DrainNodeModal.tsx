// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Typography } from "@mui/material";
import { drainClusterMember, type DrainResponse } from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";
import ConfirmDialog from "@/components/form/ConfirmDialog";

interface Props {
  open: boolean;
  nodeId: number;
  isLeader: boolean;
  onClose: () => void;
  onSuccess: (result: DrainResponse) => void;
}

const DrainNodeModal: React.FC<Props> = ({
  open,
  nodeId,
  isLeader,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();

  const handleConfirm = async () => {
    if (!accessToken) return;
    const result = await drainClusterMember(accessToken, nodeId);
    onSuccess(result);
    onClose();
  };

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={handleConfirm}
      title={`Drain node ${nodeId}?`}
      description={
        <>
          Stops new traffic on <strong>node {nodeId}</strong> and marks it
          drained immediately, so it can be restarted, upgraded, or removed. Use
          Resume to reverse.
        </>
      }
      details={{
        label: "What this does",
        content: (
          <Typography variant="body2" color="textSecondary" sx={{ mt: 1 }}>
            Sends AMF Status Indication so connected RANs treat this node&apos;s
            GUAMI as unavailable. Stops the local BGP speaker.
            {isLeader
              ? " Transfers Raft leadership to another voter once the side-effects have run."
              : ""}{" "}
            Sets <code>drainState</code> to <em>drained</em>, after which the
            node is removable.
          </Typography>
        ),
      }}
      confirmLabel="Drain"
      confirmingLabel="Draining…"
      confirmColor="warning"
      fullWidth
    />
  );
};

export default DrainNodeModal;
