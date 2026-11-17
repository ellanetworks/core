// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect } from "react";
import {
  Box,
  Button,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import { Add as AddIcon, Delete as DeleteIcon } from "@mui/icons-material";
import { useQueryClient } from "@tanstack/react-query";
import {
  useFieldArray,
  useForm,
  type FieldErrors,
  type Resolver,
} from "react-hook-form";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import SelectControl from "@/components/form/SelectControl";
import {
  currentUpdate,
  groupErrors,
  MAX_GROUP_ROLES,
  MAX_VALUE_BYTES,
  OIDC_QUERY_KEY,
  textError,
  updateOIDC,
  type OIDCSettings,
} from "@/queries/oidc";
import { RoleID, roleIDToLabel } from "@/queries/users";

const ROLE_OPTIONS = [
  { value: RoleID.Admin, label: roleIDToLabel(RoleID.Admin) },
  { value: RoleID.NetworkManager, label: roleIDToLabel(RoleID.NetworkManager) },
  { value: RoleID.ReadOnly, label: roleIDToLabel(RoleID.ReadOnly) },
] as const;

interface FormValues {
  groupsClaim: string;
  rows: { group: string; role_id: RoleID }[];
}

const resolver: Resolver<FormValues> = (values) => {
  const errors: FieldErrors<FormValues> = {};
  const claim = textError(
    values.groupsClaim.trim(),
    MAX_VALUE_BYTES,
    values.rows.length > 0
      ? "Enter the claim that holds the groups."
      : undefined,
  );
  if (claim !== undefined) {
    errors.groupsClaim = { type: "validate", message: claim };
  }
  const groups = groupErrors(values.rows.map((r) => r.group.trim()));
  if (groups.some((e) => e !== undefined)) {
    errors.rows = groups.map((message) =>
      message === undefined
        ? undefined
        : { group: { type: "validate", message } },
    ) as FieldErrors<FormValues>["rows"];
  }
  if (values.rows.length > MAX_GROUP_ROLES) {
    errors.rows = {
      type: "validate",
      message: `At most ${MAX_GROUP_ROLES} groups.`,
    } as FieldErrors<FormValues>["rows"];
  }
  return Object.keys(errors).length === 0
    ? { values, errors: {} }
    : { values: {}, errors };
};

interface EditOIDCGroupsModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  settings: OIDCSettings;
}

const EditOIDCGroupsModal: React.FC<EditOIDCGroupsModalProps> = ({
  open,
  onClose,
  onSuccess,
  settings,
}) => {
  const { accessToken } = useAuth();
  const queryClient = useQueryClient();

  const form = useForm<FormValues>({
    mode: "onChange",
    resolver,
    defaultValues: {
      groupsClaim: settings.groups_claim,
      rows: settings.group_roles.map((g) => ({ ...g })),
    },
  });
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: "rows",
  });

  useEffect(() => {
    return form.subscribe({
      formState: { values: true },
      callback: () => void form.trigger(),
    });
  }, [form]);

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    const updated = await updateOIDC(accessToken, {
      ...currentUpdate(settings),
      groups_claim: values.groupsClaim.trim(),
      group_roles: values.rows.map((r) => ({
        group: r.group.trim(),
        role_id: r.role_id,
      })),
    });
    queryClient.setQueryData(OIDC_QUERY_KEY, updated);
    void queryClient.invalidateQueries({ queryKey: ["users"] });
    void queryClient.invalidateQueries({ queryKey: ["user"] });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Roles from Groups"
      description="Optional. Users in a mapped group are created at their first sign-in. A user in several mapped groups gets the highest of their roles."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update single sign-on"
      submitLabel="Update"
      submittingLabel="Updating..."
      maxWidth="md"
    >
      <TextControl<FormValues>
        name="groupsClaim"
        label="Groups claim"
        helperText="Such as groups."
      />
      {fields.map((field, i) => (
        <Stack
          key={field.id}
          direction={{ xs: "column", sm: "row" }}
          spacing={{ xs: 0, sm: 1 }}
          sx={{
            alignItems: { xs: "stretch", sm: "start" },
            borderWidth: { xs: 1, sm: 0 },
            borderStyle: "solid",
            borderColor: "divider",
            borderRadius: 1,
            px: { xs: 1.5, sm: 0 },
            my: { xs: 1, sm: 0 },
          }}
        >
          <Box sx={{ flex: { sm: 1 } }}>
            <TextControl<FormValues>
              name={`rows.${i}.group`}
              label={`Group ${i + 1}`}
              required
            />
          </Box>
          <Box sx={{ flex: { sm: 1 } }}>
            <SelectControl<FormValues, RoleID>
              name={`rows.${i}.role_id`}
              label="Role"
              options={ROLE_OPTIONS}
              numeric
            />
          </Box>
          <Stack
            direction="row"
            sx={{ justifyContent: "flex-end", pt: { sm: 3 } }}
          >
            <Tooltip title="Remove">
              <IconButton
                aria-label={`Remove group ${i + 1}`}
                onClick={() => remove(i)}
              >
                <DeleteIcon fontSize="small" />
              </IconButton>
            </Tooltip>
          </Stack>
        </Stack>
      ))}
      <Box sx={{ mt: 1 }}>
        <Button
          startIcon={<AddIcon />}
          disabled={fields.length >= MAX_GROUP_ROLES}
          onClick={() => append({ group: "", role_id: RoleID.ReadOnly })}
        >
          Add group
        </Button>
        {fields.length >= MAX_GROUP_ROLES && (
          <Typography
            variant="body2"
            color="textSecondary"
            component="span"
            sx={{ ml: 1 }}
          >
            At most {MAX_GROUP_ROLES} groups.
          </Typography>
        )}
      </Box>
    </FormDialog>
  );
};

export default EditOIDCGroupsModal;
