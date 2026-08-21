// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { createUser, RoleID, roleIDToLabel } from "@/queries/users";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import SelectControl from "@/components/form/SelectControl";

interface CreateUserModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object({
  email: yup.string().email().required("Email is required"),
  password: yup.string().min(1).max(256).required("Password is required"),
  role_id: yup.number<RoleID>().required(),
});

type FormValues = yup.InferType<typeof schema>;

const ROLE_OPTIONS = [
  { value: RoleID.Admin, label: roleIDToLabel(RoleID.Admin) },
  { value: RoleID.NetworkManager, label: roleIDToLabel(RoleID.NetworkManager) },
  { value: RoleID.ReadOnly, label: roleIDToLabel(RoleID.ReadOnly) },
] as const;

const CreateUserModal: React.FC<CreateUserModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: { email: "", password: "", role_id: RoleID.Admin },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return;
    await createUser(
      accessToken,
      values.email,
      values.role_id,
      values.password,
    );
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Create User"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to create user"
      submitLabel="Create"
      submittingLabel="Creating..."
    >
      <TextControl<FormValues>
        name="email"
        label="Email"
        required
        autoComplete="email"
        autoFocus
      />
      <TextControl<FormValues>
        name="password"
        label="Password"
        type="password"
        required
        autoComplete="new-password"
      />
      <SelectControl<FormValues, RoleID>
        name="role_id"
        label="Role"
        options={ROLE_OPTIONS}
        numeric
      />
    </FormDialog>
  );
};

export default CreateUserModal;
