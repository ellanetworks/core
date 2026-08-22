// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import {
  updateUsageRetentionPolicy,
  type UsageRetentionPolicy,
} from "@/queries/usage";
import RetentionPolicyModal from "@/components/RetentionPolicyModal";

interface EditUsageRetentionPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: UsageRetentionPolicy;
}

const EditUsageRetentionPolicyModal: React.FC<
  EditUsageRetentionPolicyModalProps
> = ({ open, onClose, onSuccess, initialData }) => (
  <RetentionPolicyModal
    open={open}
    onClose={onClose}
    onSuccess={onSuccess}
    initialDays={initialData.days}
    title="Edit Usage Retention Policy"
    description="Set the number of days to retain usage data. After this period, data will be automatically deleted."
    itemLabel="usage data"
    errorPrefix="Failed to update usage retention policy"
    onUpdate={updateUsageRetentionPolicy}
  />
);

export default EditUsageRetentionPolicyModal;
