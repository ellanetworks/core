// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { updateRadioEventRetentionPolicy } from "@/queries/radio_events";
import RetentionPolicyModal from "@/components/RetentionPolicyModal";

interface EditRadioEventRetentionPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialDays: number;
}

const EditRadioEventRetentionPolicyModal: React.FC<
  EditRadioEventRetentionPolicyModalProps
> = ({ open, onClose, onSuccess, initialDays }) => (
  <RetentionPolicyModal
    open={open}
    onClose={onClose}
    onSuccess={onSuccess}
    initialDays={initialDays}
    title="Edit Network Log Retention Policy"
    description="Set the number of days to retain radio events. After this period, logs will be automatically deleted."
    itemLabel="radio events"
    errorPrefix="Failed to update radio event retention policy"
    onUpdate={updateRadioEventRetentionPolicy}
  />
);

export default EditRadioEventRetentionPolicyModal;
