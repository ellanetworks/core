// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateOperatorSPN } from "@/queries/operator";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";

interface EditOperatorSPNModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    fullName: string;
    shortName: string;
  };
}

const MAX_SPN_LENGTH = 50;

const schema = yup.object({
  fullName: yup
    .string()
    .required("Full name is required")
    .max(
      MAX_SPN_LENGTH,
      `Full name must be at most ${MAX_SPN_LENGTH} characters`,
    ),
  shortName: yup
    .string()
    .required("Short name is required")
    .max(
      MAX_SPN_LENGTH,
      `Short name must be at most ${MAX_SPN_LENGTH} characters`,
    ),
});

type FormValues = yup.InferType<typeof schema>;

const EditOperatorSPNModal: React.FC<EditOperatorSPNModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: {
      fullName: initialData.fullName,
      shortName: initialData.shortName,
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateOperatorSPN(accessToken, values.fullName, values.shortName);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Network Name (SPN)"
      description="The Service Provider Name (SPN) is the network name displayed on connected devices. Changes take effect for the next UE registration."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update operator SPN"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <TextControl<FormValues>
        name="fullName"
        label="Full Name"
        helperText={`The full network name shown on UE displays (max ${MAX_SPN_LENGTH} characters).`}
        slotProps={{ htmlInput: { maxLength: MAX_SPN_LENGTH } }}
        autoFocus
      />
      <TextControl<FormValues>
        name="shortName"
        label="Short Name"
        helperText={`An abbreviated network name (max ${MAX_SPN_LENGTH} characters).`}
        slotProps={{ htmlInput: { maxLength: MAX_SPN_LENGTH } }}
      />
    </FormDialog>
  );
};

export default EditOperatorSPNModal;
