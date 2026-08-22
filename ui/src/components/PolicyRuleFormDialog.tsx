// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Autocomplete, Box, TextField } from "@mui/material";
import { useController, useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { PROTOCOL_NAMES } from "@/utils/formatters";
import { isValidCidr } from "@/utils/ip";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import SelectControl from "@/components/form/SelectControl";

export const parseProtocol = (value: string): number | undefined => {
  if (!value || value.trim() === "") return undefined;
  const trimmed = value.trim().toUpperCase();
  if (/^\d+$/.test(trimmed)) {
    const num = parseInt(trimmed, 10);
    return num >= 0 && num <= 255 ? num : undefined;
  }
  const entry = Object.entries(PROTOCOL_NAMES).find(
    ([, name]) => name.toUpperCase() === trimmed,
  );
  return entry ? parseInt(entry[0], 10) : undefined;
};

const PROTOCOL_OPTIONS = Object.entries(PROTOCOL_NAMES)
  .map(([num, name]) => ({ label: `${name} (${num})`, value: name }))
  .sort((a, b) => a.label.localeCompare(b.label));

const portTest = (label: string) =>
  yup
    .string()
    .default("")
    .test("valid-port", `${label} must be between 0 and 65535`, (val) => {
      if (!val) return true;
      const num = Number(val);
      return !isNaN(num) && num >= 0 && num <= 65535;
    });

const schema = yup.object({
  description: yup
    .string()
    .required("Description is required")
    .max(256, "Description must be 256 characters or fewer"),
  action: yup
    .string()
    .oneOf(["allow", "deny"], "Invalid action")
    .required("Action is required"),
  remotePrefix: yup
    .string()
    .default("")
    .test(
      "cidr-or-empty",
      "Must be valid CIDR format (e.g., 192.168.0.0/24 or 2001:db8::/32)",
      (val) => {
        if (!val || val.trim() === "") return true;
        return isValidCidr(val);
      },
    ),
  protocol: yup
    .string()
    .default("")
    .test(
      "valid-protocol",
      "Protocol must be a valid name (tcp, udp, icmp) or number 0-255",
      (val) => {
        if (!val) return true;
        return parseProtocol(val) !== undefined;
      },
    ),
  portLow: portTest("Port Low"),
  portHigh: portTest("Port High"),
});

export type PolicyRuleFormValues = yup.InferType<typeof schema>;

export const EMPTY_RULE_FORM: PolicyRuleFormValues = {
  description: "",
  action: "allow",
  remotePrefix: "",
  protocol: "",
  portLow: "",
  portHigh: "",
};

const ACTION_OPTIONS = [
  { value: "allow", label: "Allow" },
  { value: "deny", label: "Deny" },
] as const;

interface PolicyRuleFormDialogProps {
  open: boolean;
  onClose: () => void;
  onSave: (values: PolicyRuleFormValues) => void;
  initialValues: PolicyRuleFormValues;
  isEditing: boolean;
}

const PolicyRuleFormDialog: React.FC<PolicyRuleFormDialogProps> = ({
  open,
  onClose,
  onSave,
  initialValues,
  isEditing,
}) => {
  const form = useForm<PolicyRuleFormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: initialValues,
  });

  const { field: protocolField, fieldState: protocolState } = useController({
    control: form.control,
    name: "protocol",
  });
  const showProtocolError = !!protocolState.error && protocolState.isTouched;

  const submit = async (values: PolicyRuleFormValues) => {
    onSave(values);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      title={isEditing ? "Edit Rule" : "Add Rule"}
      form={form}
      onSubmit={submit}
      submitLabel={isEditing ? "Update" : "Add"}
      submittingLabel={isEditing ? "Update" : "Add"}
      fullWidth
    >
      <TextControl<PolicyRuleFormValues>
        name="description"
        label="Description"
        helperText="A short label for this rule"
        autoFocus
      />
      <SelectControl<PolicyRuleFormValues, string>
        name="action"
        label="Action"
        options={ACTION_OPTIONS}
      />
      <TextControl<PolicyRuleFormValues>
        name="remotePrefix"
        label="Remote Prefix (CIDR)"
        placeholder="e.g., 192.168.0.0/24"
        helperText="Optional — IPv4 or IPv6 CIDR (e.g., 10.0.0.0/8 or 2001:db8::/32)"
      />
      <Autocomplete
        fullWidth
        options={PROTOCOL_OPTIONS}
        getOptionLabel={(option) => option.label}
        value={
          PROTOCOL_OPTIONS.find(
            (option) => option.value === protocolField.value,
          ) ?? null
        }
        onChange={(_event, value) => protocolField.onChange(value?.value ?? "")}
        onBlur={protocolField.onBlur}
        isOptionEqualToValue={(option, value) => option.value === value.value}
        renderInput={(params) => (
          <TextField
            {...params}
            label="Protocol"
            placeholder="Search protocols..."
            error={showProtocolError}
            helperText={
              showProtocolError
                ? protocolState.error?.message
                : "Optional – search or leave empty for any"
            }
            margin="normal"
          />
        )}
      />
      <Box sx={{ display: "flex", gap: 2 }}>
        <TextControl<PolicyRuleFormValues>
          name="portLow"
          label="Port Low"
          type="number"
          placeholder="0-65535"
          helperText="Optional — applies to TCP/UDP only"
          sx={{ flex: 1 }}
        />
        <TextControl<PolicyRuleFormValues>
          name="portHigh"
          label="Port High"
          type="number"
          placeholder="0-65535"
          helperText="Optional — applies to TCP/UDP only"
          sx={{ flex: 1 }}
        />
      </Box>
    </FormDialog>
  );
};

export default PolicyRuleFormDialog;
