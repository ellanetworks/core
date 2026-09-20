// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { setClusterMemberDisplayName } from "@/queries/cluster";
import { NodeId } from "@/queries/nodeId";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";

interface Props {
  open: boolean;
  nodeId: NodeId;
  initialDisplayName: string;
  onClose: () => void;
  onSuccess: () => void;
}

const MAX_DISPLAY_NAME_LENGTH = 63;

const schema = yup.object({
  displayName: yup
    .string()
    .default("")
    .max(
      MAX_DISPLAY_NAME_LENGTH,
      `Name must be at most ${MAX_DISPLAY_NAME_LENGTH} characters`,
    ),
});

type FormValues = yup.InferType<typeof schema>;

const RenameNodeModal: React.FC<Props> = ({
  open,
  nodeId,
  initialDisplayName,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: { displayName: initialDisplayName },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await setClusterMemberDisplayName(accessToken, nodeId, values.displayName);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Rename node"
      description={`Sets the name shown for ${nodeId} across the UI. The name is for operators; every action still targets the node identity.`}
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to set display name"
      submitLabel="Save"
      submittingLabel="Saving..."
      fullWidth
    >
      <TextControl<FormValues>
        name="displayName"
        label="Display name"
        helperText={`Leave empty to show the node identity instead (max ${MAX_DISPLAY_NAME_LENGTH} characters).`}
        slotProps={{ htmlInput: { maxLength: MAX_DISPLAY_NAME_LENGTH } }}
        autoFocus
      />
    </FormDialog>
  );
};

export default RenameNodeModal;
