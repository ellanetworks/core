// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import {
  updateAuditLogRetentionPolicy,
  type AuditLogRetentionPolicy,
} from "@/queries/audit_logs";
import RetentionPolicyModal from "@/components/RetentionPolicyModal";

interface EditAuditLogRetentionPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: AuditLogRetentionPolicy;
}

const EditAuditLogRetentionPolicyModal: React.FC<
  EditAuditLogRetentionPolicyModalProps
> = ({ open, onClose, onSuccess, initialData }) => (
  <RetentionPolicyModal
    open={open}
    onClose={onClose}
    onSuccess={onSuccess}
    initialDays={initialData.days}
    title="Edit Audit Log Retention Policy"
    description="Set the number of days to retain audit logs. After this period, logs will be automatically deleted."
    itemLabel="logs"
    errorPrefix="Failed to update audit log retention policy"
    onUpdate={updateAuditLogRetentionPolicy}
  />
);

export default EditAuditLogRetentionPolicyModal;
