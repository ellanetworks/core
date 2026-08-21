// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Alert } from "@mui/material";
import { useForm, useWatch } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import NumberControl from "@/components/form/NumberControl";

export const MIN_RETENTION_DAYS = 1;
export const MAX_RETENTION_DAYS = 3650;

const schema = yup.object({
  days: yup
    .number()
    .typeError("Days must be a number")
    .integer("Days must be a whole number")
    .min(MIN_RETENTION_DAYS, `Minimum retention is ${MIN_RETENTION_DAYS} day`)
    .max(
      MAX_RETENTION_DAYS,
      `Maximum retention is ${MAX_RETENTION_DAYS} days (10 years)`,
    )
    .required("Days is required"),
});

type FormValues = yup.InferType<typeof schema>;

interface RetentionPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialDays: number;
  title: string;
  description: string;
  itemLabel: string;
  errorPrefix: string;
  onUpdate: (authToken: string, days: number) => Promise<unknown>;
}

const RetentionPolicyModal: React.FC<RetentionPolicyModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialDays,
  title,
  description,
  itemLabel,
  errorPrefix,
  onUpdate,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onChange",
    resolver: yupResolver(schema),
    values: { days: initialDays },
  });

  const days = useWatch({ control: form.control, name: "days" });
  const isReduction =
    typeof days === "number" &&
    Number.isFinite(days) &&
    days >= MIN_RETENTION_DAYS &&
    days < initialDays;

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await onUpdate(accessToken, values.days);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title={title}
      description={description}
      form={form}
      onSubmit={submit}
      errorPrefix={errorPrefix}
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      {isReduction && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          Reducing retention from {initialDays} to {days} days will permanently
          delete {itemLabel} older than {days} day
          {days === 1 ? "" : "s"}.
        </Alert>
      )}
      <NumberControl<FormValues>
        name="days"
        label="Days"
        min={MIN_RETENTION_DAYS}
        max={MAX_RETENTION_DAYS}
        helperText={`${MIN_RETENTION_DAYS} to ${MAX_RETENTION_DAYS} days`}
        showErrorWhileTyping
        autoFocus
      />
    </FormDialog>
  );
};

export default RetentionPolicyModal;
