// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Alert, Box } from "@mui/material";
import { NodeId } from "@/queries/nodeId";
import { drainClusterMember, type DrainResponse } from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";
import ConfirmDialog from "@/components/form/ConfirmDialog";
import NodeIdentity from "@/components/NodeIdentity";

interface Props {
  open: boolean;
  nodeId: NodeId;
  nodeLabel: string;
  isLastActive: boolean;
  isLeader: boolean;
  isSelf: boolean;
  onClose: () => void;
  onSuccess: (result: DrainResponse) => void;
}

const DrainNodeModal: React.FC<Props> = ({
  open,
  nodeId,
  nodeLabel,
  isLastActive,
  isLeader,
  isSelf,
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
      title={`Drain node ${nodeLabel}?`}
      description={
        <>
          Stops new traffic on <strong>node {nodeLabel}</strong> and moves its
          subscribers to the rest of the cluster.
        </>
      }
      extra={
        <>
          <NodeIdentity nodeId={nodeId} />
          {(isLastActive || isLeader || isSelf) && (
            <Box
              sx={{ mt: 2, display: "flex", flexDirection: "column", gap: 1 }}
            >
              {isLastActive && (
                <Alert severity="warning">
                  No other node is active, so subscribers have nowhere to move
                  and the drain will not finish.
                </Alert>
              )}
              {isLeader && (
                <Alert severity="info">
                  This node is the leader. Leadership will move to another node.
                </Alert>
              )}
              {isSelf && (
                <Alert severity="info">
                  This is the node serving this page. It stops taking new
                  traffic; your session is unaffected.
                </Alert>
              )}
            </Box>
          )}
        </>
      }
      confirmLabel="Drain"
      confirmingLabel="Draining…"
      confirmColor="warning"
      fullWidth
    />
  );
};

export default DrainNodeModal;
