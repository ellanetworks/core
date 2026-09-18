// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
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
          Stops new traffic on <strong>node {nodeId}</strong> and moves its
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
