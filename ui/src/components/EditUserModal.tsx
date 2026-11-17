// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { useQuery } from "@tanstack/react-query";
import { updateUser, RoleID, APIUser, roleIDToLabel } from "@/queries/users";
import { getOIDC, OIDC_QUERY_KEY, roleFromGroups } from "@/queries/oidc";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import SelectControl from "@/components/form/SelectControl";

interface EditUserModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APIUser;
}

interface FormValues {
  email: string;
  role: RoleID;
}

const ROLE_OPTIONS = [
  { value: RoleID.Admin, label: roleIDToLabel(RoleID.Admin) },
  { value: RoleID.NetworkManager, label: roleIDToLabel(RoleID.NetworkManager) },
  { value: RoleID.ReadOnly, label: roleIDToLabel(RoleID.ReadOnly) },
] as const;

const EditUserModal: React.FC<EditUserModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken, authReady } = useAuth();

  const oidc = useQuery({
    queryKey: OIDC_QUERY_KEY,
    queryFn: () => getOIDC(accessToken!),
    enabled: authReady && !!accessToken,
  }).data;

  const form = useForm<FormValues>({
    values: {
      email: initialData.email,
      role: initialData.role_id,
    },
  });

  const fromGroups = roleFromGroups(initialData, oidc);

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateUser(accessToken, values.email, values.role);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit User"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update user"
      submitLabel="Update"
      submittingLabel="Updating..."
    >
      <TextControl<FormValues> name="email" label="Email" disabled />
      <SelectControl<FormValues, RoleID>
        name="role"
        label="Role"
        options={ROLE_OPTIONS}
        numeric
        autoFocus
        disabled={fromGroups}
      />
    </FormDialog>
  );
};

export default EditUserModal;
