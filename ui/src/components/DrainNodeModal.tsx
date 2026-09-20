// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { NodeId } from "@/queries/nodeId";
import { drainClusterMember, type DrainResponse } from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";
import ConfirmDialog from "@/components/form/ConfirmDialog";

interface Props {
  open: boolean;
  nodeId: NodeId;
  nodeLabel: string;
  onClose: () => void;
  onSuccess: (result: DrainResponse) => void;
}

const DrainNodeModal: React.FC<Props> = ({
  open,
  nodeId,
  nodeLabel,
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
      confirmLabel="Drain"
      confirmingLabel="Draining…"
      confirmColor="warning"
      fullWidth
    />
  );
};

export default DrainNodeModal;
