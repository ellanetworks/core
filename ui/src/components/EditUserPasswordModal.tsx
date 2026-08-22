// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateUserPassword } from "@/queries/users";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog, { stripStatusPrefix } from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";

const schema = yup.object({
  email: yup.string().required(),
  password: yup
    .string()
    .min(1, "Password is required")
    .required("Password is required"),
});

type FormValues = yup.InferType<typeof schema>;

interface EditUserPasswordModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    email: string;
  };
}

const EditUserPasswordModal: React.FC<EditUserPasswordModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: { email: initialData.email, password: "" },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateUserPassword(accessToken, values.email, values.password);
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
      <TextControl<FormValues> name="email" label="Email" disabled />
      <TextControl<FormValues>
        name="password"
        label="New Password"
        type="password"
        required
        autoComplete="new-password"
        autoFocus
      />
    </FormDialog>
  );
};

export default EditUserPasswordModal;
