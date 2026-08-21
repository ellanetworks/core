// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Typography } from "@mui/material";
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
          Clears drain state on <strong>node {nodeId}</strong> and restarts its
          local BGP speaker (if BGP is enabled). Route advertisements resume on
          the next reconciler tick.
        </>
      }
      details={{
        label: "What Resume does not reverse",
        content: (
          <Typography
            component="ul"
            variant="body2"
            color="textSecondary"
            sx={{ mt: 1, pl: 2.5 }}
          >
            <li>
              The AMF Status Indication sent at drain time — RANs treat this
              node&apos;s GUAMI as available again only after the next NG Setup
              (typically on RAN restart or SCTP reconnect).
            </li>
            <li>
              Raft leadership transfer — if this node was the leader when
              drained, it stays a follower until something else moves
              leadership.
            </li>
          </Typography>
        ),
      }}
      confirmLabel="Resume"
      confirmingLabel="Resuming…"
      confirmColor="primary"
      fullWidth
    />
  );
};

export default ResumeNodeModal;
