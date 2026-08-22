// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import ConfirmDialog from "@/components/form/ConfirmDialog";

interface ConfirmationModalProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
  title: string;
  description: string;
}

const DeleteConfirmationModal: React.FC<ConfirmationModalProps> = ({
  open,
  onClose,
  onConfirm,
  title,
  description,
}) => (
  <ConfirmDialog
    open={open}
    onClose={onClose}
    onConfirm={onConfirm}
    title={title}
    description={description}
    confirmLabel="Confirm"
    confirmingLabel="Deleting…"
    confirmColor="error"
  />
);

export default DeleteConfirmationModal;
