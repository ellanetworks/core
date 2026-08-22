// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Checkbox, FormControlLabel, Stack } from "@mui/material";
import { LocalizationProvider } from "@mui/x-date-pickers/LocalizationProvider";
import { AdapterDayjs } from "@mui/x-date-pickers/AdapterDayjs";
import { DatePicker } from "@mui/x-date-pickers/DatePicker";
import dayjs, { Dayjs } from "dayjs";
import { useController, useForm, useWatch } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { createAPIToken, createUserAPIToken } from "@/queries/api_tokens";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";

interface CreateAPITokenModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: (token: string) => void;
  targetEmail?: string;
}

const schema = yup.object({
  name: yup
    .string()
    .trim()
    .min(3, "Name must be at least 3 characters")
    .max(50, "Name must be at most 50 characters")
    .required("Name is required"),
  noExpiry: yup.boolean().required(),
  expiry: yup
    .mixed<Dayjs>()
    .nullable()
    .test("expiry-required", "Expiry date is required", function (value) {
      const { noExpiry } = this.parent as { noExpiry: boolean };
      return noExpiry ? true : !!value;
    })
    .test(
      "expiry-future",
      "Expiry date must be in the future",
      function (value) {
        const { noExpiry } = this.parent as { noExpiry: boolean };
        if (noExpiry || !value) return true;
        return value.isAfter(dayjs().startOf("day"));
      },
    ),
});

type FormValues = yup.InferType<typeof schema>;

const CreateAPITokenModal: React.FC<CreateAPITokenModalProps> = ({
  open,
  onClose,
  onSuccess,
  targetEmail,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: { name: "", noExpiry: false, expiry: null },
  });

  const { field: expiryField, fieldState: expiryState } = useController({
    control: form.control,
    name: "expiry",
  });
  const { field: noExpiryField } = useController({
    control: form.control,
    name: "noExpiry",
  });
  const noExpiry = useWatch({ control: form.control, name: "noExpiry" });

  const showExpiryError =
    !!expiryState.error && expiryState.isTouched && !noExpiry;

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;

    const expiryISO =
      values.noExpiry || !values.expiry
        ? ""
        : values.expiry.toDate().toISOString();

    const res = targetEmail
      ? await createUserAPIToken(
          accessToken,
          targetEmail,
          values.name.trim(),
          expiryISO,
        )
      : await createAPIToken(accessToken, values.name.trim(), expiryISO);

    onSuccess(res.token);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      title="Create API Token"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to create API token"
      submitLabel="Create"
      submittingLabel="Creating..."
      fullWidth={false}
    >
      <Stack spacing={2} sx={{ mt: 1 }}>
        <TextControl<FormValues>
          name="name"
          label="Name"
          helperText="3–50 characters"
          placeholder="e.g., CI Pipeline, Local Script"
          autoFocus
        />

        <LocalizationProvider dateAdapter={AdapterDayjs}>
          <DatePicker
            label="Expiry date"
            value={expiryField.value ?? null}
            onChange={(value) => expiryField.onChange(value)}
            onClose={expiryField.onBlur}
            disabled={noExpiry}
            minDate={dayjs().startOf("day")}
            slotProps={{
              textField: {
                fullWidth: true,
                error: showExpiryError,
                helperText: showExpiryError ? expiryState.error?.message : " ",
              },
            }}
          />
        </LocalizationProvider>
      </Stack>

      <FormControlLabel
        sx={{ mt: -0.5 }}
        control={
          <Checkbox
            checked={!!noExpiryField.value}
            onChange={(event) => noExpiryField.onChange(event.target.checked)}
            onBlur={noExpiryField.onBlur}
          />
        }
        label="No expiry"
      />
    </FormDialog>
  );
};

export default CreateAPITokenModal;
