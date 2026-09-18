// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { resumeClusterMember } from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";
import ConfirmDialog from "@/components/form/ConfirmDialog";

interface Props {
  open: boolean;
  nodeId: number;
  onClose: () => void;
  onSuccess: () => void;
}

const ResumeNodeModal: React.FC<Props> = ({
  open,
  nodeId,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();

  const handleConfirm = async () => {
    if (!accessToken) return;
    await resumeClusterMember(accessToken, nodeId);
    onSuccess();
    onClose();
  };

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={handleConfirm}
      title={`Resume node ${nodeId}?`}
      description={
        <>
          Clears drain state on <strong>node {nodeId}</strong>.
        </>
      }
      confirmLabel="Resume"
      confirmingLabel="Resuming…"
      confirmColor="primary"
      fullWidth
    />
  );
};

export default ResumeNodeModal;
