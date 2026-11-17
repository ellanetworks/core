// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
import { useQuery } from "@tanstack/react-query";
import { updateUser, RoleID, APIUser, roleIDToLabel } from "@/queries/users";
import { getOIDC, OIDC_QUERY_KEY, roleFromGroups } from "@/queries/oidc";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import SelectControl from "@/components/form/SelectControl";
import PinRoleField from "@/components/PinRoleField";

interface EditUserModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APIUser;
}

interface FormValues {
  email: string;
  role: RoleID;
  pinned: boolean;
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
      pinned: initialData.role_pinned,
    },
  });

  const role = useWatch({ control: form.control, name: "role" });
  const pinnable =
    (oidc?.group_roles.length ?? 0) > 0 || initialData.role_pinned;
  const pinRequired =
    roleFromGroups(initialData, oidc) && role !== initialData.role_id;

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateUser(
      accessToken,
      values.email,
      values.role,
      pinnable ? values.pinned || pinRequired : undefined,
    );
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
      />
      {pinnable && (
        <Controller
          name="pinned"
          control={form.control}
          render={({ field }) => (
            <PinRoleField
              checked={field.value || pinRequired}
              onChange={field.onChange}
              required={pinRequired}
            />
          )}
        />
      )}
    </FormDialog>
  );
};

export default EditUserModal;
