// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateSubscriber } from "@/queries/subscribers";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  msisdnSchema,
  normalizeMSISDN,
  type EditSubscriberFields,
} from "@/components/subscriberIdentity";

interface EditSubscriberMSISDNModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: EditSubscriberFields;
}

const schema = yup.object({
  msisdn: msisdnSchema,
});

type FormValues = yup.InferType<typeof schema>;

const EditSubscriberMSISDNModal: React.FC<EditSubscriberMSISDNModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: { msisdn: initialData.msisdn },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateSubscriber(
      accessToken,
      initialData.imsi,
      initialData.profileName,
      initialData.description,
      normalizeMSISDN(values.msisdn),
    );
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit MSISDN"
      description="The subscriber's phone number. SMS requires an MSISDN."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update subscriber"
      submitLabel="Update"
      submittingLabel="Updating..."
    >
      <TextControl<FormValues>
        name="msisdn"
        label="MSISDN"
        placeholder="+15551230001"
        helperText="E.164 format, for example +15551230001. Leave blank to remove it."
        autoFocus
      />
    </FormDialog>
  );
};

export default EditSubscriberMSISDNModal;
