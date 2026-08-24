// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useId, useState } from "react";
import {
  Alert,
  Button,
  Collapse,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from "@mui/material";
import type { DialogProps } from "@mui/material";
import {
  FormProvider,
  useFormState,
  type FieldValues,
  type UseFormReturn,
} from "react-hook-form";

export const defaultFormatError = (error: unknown): string =>
  error instanceof Error ? error.message : "Unknown error occurred.";

export const stripStatusPrefix = (error: unknown): string =>
  defaultFormatError(error).replace(/^\d{3}: [A-Za-z ]+\.\s*/, "");

interface FormDialogProps<T extends FieldValues> {
  open: boolean;
  onClose: () => void;
  onSuccess?: () => void;
  title: string;
  description?: string;
  form: UseFormReturn<T>;
  onSubmit: (values: T) => Promise<void | false>;
  errorPrefix?: string;
  formatError?: (error: unknown) => string;
  submitLabel: string;
  submittingLabel: string;
  maxWidth?: DialogProps["maxWidth"];
  fullWidth?: boolean;
  children: React.ReactNode;
}

const FormDialog = <T extends FieldValues>({
  open,
  onClose,
  onSuccess,
  title,
  description,
  form,
  onSubmit,
  errorPrefix,
  formatError = defaultFormatError,
  submitLabel,
  submittingLabel,
  maxWidth = "sm",
  fullWidth = true,
  children,
}: FormDialogProps<T>) => {
  const baseId = useId();
  const titleId = `${baseId}-title`;
  const descriptionId = `${baseId}-description`;
  const { isValid, isSubmitting } = useFormState({ control: form.control });
  const [submitError, setSubmitError] = useState("");

  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setSubmitError("");
  }

  useEffect(() => {
    if (open) form.reset();
  }, [open, form]);

  const submit = form.handleSubmit(async (values) => {
    setSubmitError("");
    try {
      if ((await onSubmit(values)) === false) return;
    } catch (error: unknown) {
      const message = formatError(error);
      setSubmitError(errorPrefix ? `${errorPrefix}: ${message}` : message);
      return;
    }
    onClose();
    onSuccess?.();
  });

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby={titleId}
      aria-describedby={description ? descriptionId : undefined}
      fullWidth={fullWidth}
      maxWidth={maxWidth}
    >
      <DialogTitle id={titleId}>{title}</DialogTitle>
      <FormProvider {...form}>
        <form onSubmit={submit} noValidate>
          <DialogContent dividers>
            {description && (
              <DialogContentText id={descriptionId} sx={{ mb: 2 }}>
                {description}
              </DialogContentText>
            )}
            <Collapse in={!!submitError}>
              <Alert
                onClose={() => setSubmitError("")}
                sx={{ mb: 2 }}
                severity="error"
              >
                {submitError}
              </Alert>
            </Collapse>
            {children}
          </DialogContent>
          <DialogActions>
            <Button onClick={onClose}>Cancel</Button>
            <Button
              type="submit"
              variant="contained"
              color="success"
              disabled={!isValid || isSubmitting}
            >
              {isSubmitting ? submittingLabel : submitLabel}
            </Button>
          </DialogActions>
        </form>
      </FormProvider>
    </Dialog>
  );
};

export default FormDialog;
