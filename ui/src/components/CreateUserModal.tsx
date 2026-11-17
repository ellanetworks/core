// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { FormControlLabel, Switch } from "@mui/material";
import { Controller, useForm, useWatch } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import { useQuery } from "@tanstack/react-query";
import * as yup from "yup";
import { createUser, RoleID, roleIDToLabel } from "@/queries/users";
import { getOIDC, OIDC_QUERY_KEY } from "@/queries/oidc";
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
  with_password: yup.boolean().required(),
  password: yup
    .string()
    .max(256)
    .when(["with_password", "$ssoConfigured"], ([withPassword, sso], s) =>
      !sso || withPassword
        ? s.min(1).required("Password is required")
        : s.optional(),
    ),
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
  const { accessToken, authReady } = useAuth();

  const oidc = useQuery({
    queryKey: OIDC_QUERY_KEY,
    queryFn: () => getOIDC(accessToken!),
    enabled: authReady && !!accessToken,
  }).data;
  const ssoConfigured =
    oidc !== undefined && oidc.issuer !== "" && oidc.client_id !== "";

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    context: { ssoConfigured },
    defaultValues: {
      email: "",
      with_password: true,
      password: "",
      role_id: RoleID.Admin,
    },
  });

  const withPassword = useWatch({
    control: form.control,
    name: "with_password",
  });
  const needsPassword = !ssoConfigured || withPassword;

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await createUser(
      accessToken,
      values.email,
      values.role_id,
      needsPassword ? (values.password ?? "") : "",
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
      {ssoConfigured && (
        <Controller
          name="with_password"
          control={form.control}
          render={({ field }) => (
            <FormControlLabel
              sx={{ mt: 1 }}
              control={
                <Switch
                  checked={field.value}
                  onChange={(e) => {
                    field.onChange(e.target.checked);
                    void form.trigger("password");
                  }}
                />
              }
              label="Password"
            />
          )}
        />
      )}
      {needsPassword && (
        <TextControl<FormValues>
          name="password"
          label="Password"
          type="password"
          required
          autoComplete="new-password"
        />
      )}
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
