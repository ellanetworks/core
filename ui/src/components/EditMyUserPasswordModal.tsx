// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateMyUserPassword } from "@/queries/users";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog, { stripStatusPrefix } from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";

const schema = yup.object({
  currentPassword: yup
    .string()
    .min(1, "Current password is required")
    .required("Current password is required"),
  password: yup
    .string()
    .min(1, "New password is required")
    .required("New password is required"),
});

type FormValues = yup.InferType<typeof schema>;

interface EditMyUserPasswordModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const EditMyUserPasswordModal: React.FC<EditMyUserPasswordModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: { currentPassword: "", password: "" },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateMyUserPassword(
      accessToken,
      values.currentPassword,
      values.password,
    );
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Change Password"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update password"
      formatError={stripStatusPrefix}
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <TextControl<FormValues>
        name="currentPassword"
        label="Current Password"
        type="password"
        required
        autoComplete="current-password"
        autoFocus
      />
      <TextControl<FormValues>
        name="password"
        label="New Password"
        type="password"
        required
        autoComplete="new-password"
      />
    </FormDialog>
  );
};

export default EditMyUserPasswordModal;
