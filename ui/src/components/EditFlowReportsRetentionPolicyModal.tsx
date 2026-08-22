// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import {
  updateFlowReportsRetentionPolicy,
  type FlowReportsRetentionPolicy,
} from "@/queries/flow_reports";
import RetentionPolicyModal from "@/components/RetentionPolicyModal";

interface EditFlowReportsRetentionPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: FlowReportsRetentionPolicy;
}

const EditFlowReportsRetentionPolicyModal: React.FC<
  EditFlowReportsRetentionPolicyModalProps
> = ({ open, onClose, onSuccess, initialData }) => (
  <RetentionPolicyModal
    open={open}
    onClose={onClose}
    onSuccess={onSuccess}
    initialDays={initialData.days}
    title="Edit Flow Reports Retention Policy"
    description="Set the number of days to retain flow report data. After this period, data will be automatically deleted."
    itemLabel="flow report data"
    errorPrefix="Failed to update flow reports retention policy"
    onUpdate={updateFlowReportsRetentionPolicy}
  />
);

export default EditFlowReportsRetentionPolicyModal;
