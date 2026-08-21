// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Box, Button, IconButton, TextField, Typography } from "@mui/material";
import { Add as AddIcon, Delete as DeleteIcon } from "@mui/icons-material";
import { useController, useForm } from "react-hook-form";
import type { Control } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { useAuth } from "@/contexts/AuthContext";
import { createFramedRoute, updateFramedRoute } from "@/queries/data_networks";
import { isValidCidr } from "@/utils/ip";
import FormDialog from "@/components/form/FormDialog";
import SubscriberSelectField from "@/components/SubscriberSelectField";

const MAX_PER_FAMILY = 8;

interface FramedRouteEdit {
  imsi: string;
  ipv4: string[];
  ipv6: string[];
}

interface CreateFramedRouteModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  dataNetwork: string;
  edit?: FramedRouteEdit;
}

const clean = (values: string[]) =>
  values.map((value) => value.trim()).filter((value) => value !== "");

const prefixList = () =>
  yup
    .array()
    .of(yup.string().defined())
    .defined()
    .test(
      "valid-prefixes",
      "Enter a valid CIDR prefix.",
      (values) =>
        !(values ?? []).some(
          (value) => value.trim() !== "" && !isValidCidr(value.trim()),
        ),
    )
    .test(
      "within-cap",
      `Up to ${MAX_PER_FAMILY} prefixes per family.`,
      (values) => clean(values ?? []).length <= MAX_PER_FAMILY,
    );

const schema = yup
  .object({
    imsi: yup.string().trim().required(),
    ipv4: prefixList(),
    ipv6: prefixList(),
  })
  .test(
    "at-least-one-prefix",
    "Add at least one prefix.",
    (values) =>
      clean(values.ipv4 ?? []).length + clean(values.ipv6 ?? []).length > 0,
  );

type FormValues = yup.InferType<typeof schema>;

interface PrefixListFieldProps {
  control: Control<FormValues>;
  name: "ipv4" | "ipv6";
  title: string;
  addLabel: string;
  placeholder: string;
}

const PrefixListField: React.FC<PrefixListFieldProps> = ({
  control,
  name,
  title,
  addLabel,
  placeholder,
}) => {
  const { field } = useController({ control, name });
  const values: string[] = field.value ?? [];

  return (
    <Box sx={{ mt: 2 }}>
      <Typography variant="subtitle2" sx={{ mb: 1 }}>
        {title}
      </Typography>
      {values.map((value, index) => {
        const invalid = value.trim() !== "" && !isValidCidr(value.trim());
        return (
          <Box
            key={index}
            sx={{ display: "flex", alignItems: "flex-start", gap: 1, mb: 1 }}
          >
            <TextField
              fullWidth
              size="small"
              value={value}
              onChange={(event) =>
                field.onChange(
                  values.map((current, i) =>
                    i === index ? event.target.value : current,
                  ),
                )
              }
              placeholder={placeholder}
              error={invalid}
              helperText={invalid ? "Enter a valid CIDR prefix." : ""}
            />
            <IconButton
              aria-label="Remove prefix"
              onClick={() =>
                field.onChange(values.filter((_, i) => i !== index))
              }
            >
              <DeleteIcon fontSize="small" />
            </IconButton>
          </Box>
        );
      })}
      <Button
        size="small"
        startIcon={<AddIcon />}
        onClick={() => field.onChange([...values, ""])}
        disabled={values.length >= MAX_PER_FAMILY}
      >
        {addLabel}
      </Button>
    </Box>
  );
};

const CreateFramedRouteModal: React.FC<CreateFramedRouteModalProps> = ({
  open,
  onClose,
  onSuccess,
  dataNetwork,
  edit,
}) => {
  const { accessToken } = useAuth();
  const isEdit = !!edit;

  const form = useForm<FormValues>({
    mode: "onChange",
    resolver: yupResolver(schema),
    values: {
      imsi: edit?.imsi ?? "",
      ipv4: edit?.ipv4 ?? [],
      ipv6: edit?.ipv6 ?? [],
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    const ipv4 = clean(values.ipv4 ?? []);
    const ipv6 = clean(values.ipv6 ?? []);

    if (isEdit && edit) {
      await updateFramedRoute(accessToken, dataNetwork, edit.imsi, ipv4, ipv6);
    } else {
      await createFramedRoute(
        accessToken,
        dataNetwork,
        values.imsi.trim(),
        ipv4,
        ipv6,
      );
    }
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title={isEdit ? "Edit Framed Routes" : "Add Framed Routes"}
      form={form}
      onSubmit={submit}
      submitLabel="Save"
      submittingLabel="Saving..."
      fullWidth
    >
      <SubscriberSelectField
        control={form.control}
        name="imsi"
        dataNetwork={dataNetwork}
        open={open}
        readOnly={isEdit}
      />
      <PrefixListField
        control={form.control}
        name="ipv4"
        title="IPv4 prefixes"
        addLabel="Add IPv4 prefix"
        placeholder="e.g., 192.168.60.0/24"
      />
      <PrefixListField
        control={form.control}
        name="ipv6"
        title="IPv6 prefixes"
        addLabel="Add IPv6 prefix"
        placeholder="e.g., fd00:60::/64"
      />
      <Typography
        variant="caption"
        color="textSecondary"
        sx={{ mt: 2, display: "block" }}
      >
        Up to {MAX_PER_FAMILY} prefixes per family. NAT must be disabled to use
        framed routes.
      </Typography>
    </FormDialog>
  );
};

export default CreateFramedRouteModal;
