// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import ConfirmDialog from "@/components/form/ConfirmDialog";
import { useAuth } from "@/contexts/AuthContext";
import { deregisterSubscriber } from "@/queries/subscribers";

interface DeregisterRoamerModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  imsi: string;
}

const DeregisterRoamerModal: React.FC<DeregisterRoamerModalProps> = ({
  open,
  onClose,
  onSuccess,
  imsi,
}) => {
  const { accessToken } = useAuth();

  const confirm = async () => {
    if (!accessToken) return;
    await deregisterSubscriber(accessToken, imsi);
    onClose();
    onSuccess();
  };

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={confirm}
      title="Confirm Deregistration"
      description={`Are you sure you want to deregister the roamer "${imsi}"?`}
      confirmLabel="Deregister"
      confirmingLabel="Deregistering…"
    />
  );
};

export default DeregisterRoamerModal;
