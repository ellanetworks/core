// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateOperatorSMS } from "@/queries/operator";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import { msisdnSchema } from "@/components/subscriberIdentity";
import { PRODUCT } from "@/utils/product";

interface EditOperatorSMSModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: { smsNumber: string };
}

const schema = yup.object({
  smsNumber: msisdnSchema.test(
    "required",
    "SMS number is required",
    (v) => !!v?.trim(),
  ),
});

type FormValues = yup.InferType<typeof schema>;

const EditOperatorSMSModal: React.FC<EditOperatorSMSModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();
  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: { smsNumber: initialData.smsNumber },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateOperatorSMS(accessToken, {
      smsNumber: values.smsNumber.trim(),
    });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="SMS Number"
      description={`${PRODUCT.name}'s own E.164 number, given to SMSCs when they deliver messages to your subscribers.`}
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update SMS settings"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <TextControl<FormValues>
        name="smsNumber"
        label="E.164 Number"
        placeholder="+15550001111"
        helperText="For example +15550001111."
        autoFocus
      />
    </FormDialog>
  );
};

export default EditOperatorSMSModal;
