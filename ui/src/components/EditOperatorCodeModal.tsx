// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Alert } from "@mui/material";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateOperatorCode } from "@/queries/operator";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";

interface EditOperatorCodeModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object({
  operatorCode: yup
    .string()
    .required("Operator Code is required.")
    .matches(
      /^[0-9A-Fa-f]{32}$/,
      "Operator Code must be a 32-character hexadecimal string.",
    ),
});

type FormValues = yup.InferType<typeof schema>;

const EditOperatorCodeModal: React.FC<EditOperatorCodeModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: { operatorCode: "" },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateOperatorCode(accessToken, values.operatorCode);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Operator Code"
      description="The Operator Code (OP) is a secret identifier used to authenticate the operator and provision SIM cards. Keep this code secure as it can't be retrieved once set."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update operator code"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <Alert severity="warning" sx={{ mt: 2, mb: 2 }}>
        This operation cannot be undone. The operator code cannot be retrieved
        once set.
      </Alert>
      <TextControl<FormValues>
        name="operatorCode"
        label="Operator Code"
        autoFocus
      />
    </FormDialog>
  );
};

export default EditOperatorCodeModal;
