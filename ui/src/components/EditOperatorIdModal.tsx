// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateOperatorID } from "@/queries/operator";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";

interface EditOperatorIdModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    mcc: string;
    mnc: string;
  };
}

const schema = yup.object({
  mcc: yup
    .string()
    .matches(/^\d{3}$/, "MCC must be a 3 decimal digit")
    .required("MCC is required"),
  mnc: yup
    .string()
    .matches(/^\d{2,3}$/, "MNC must be a 2 or 3 decimal digit")
    .required("MNC is required"),
});

type FormValues = yup.InferType<typeof schema>;

const EditOperatorIdModal: React.FC<EditOperatorIdModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: { mcc: initialData.mcc, mnc: initialData.mnc },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateOperatorID(accessToken, values.mcc, values.mnc);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Operator ID"
      description="The Operator ID is a combination of Mobile Country Code (MCC) and Mobile Network Code (MNC). The Operator ID is used to uniquely identify the operator in the network."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update operator ID"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <TextControl<FormValues> name="mcc" label="MCC" autoFocus />
      <TextControl<FormValues> name="mnc" label="MNC" />
    </FormDialog>
  );
};

export default EditOperatorIdModal;
