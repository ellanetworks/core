// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  updateSubscriber,
  MAX_DESCRIPTION_LENGTH,
} from "@/queries/subscribers";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  descriptionSchema,
  type EditSubscriberFields,
} from "@/components/subscriberIdentity";

interface EditSubscriberDescriptionModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: EditSubscriberFields;
}

const schema = yup.object({
  description: descriptionSchema,
});

type FormValues = yup.InferType<typeof schema>;

const EditSubscriberDescriptionModal: React.FC<
  EditSubscriberDescriptionModalProps
> = ({ open, onClose, onSuccess, initialData }) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: { description: initialData.description },
  });

  // The API replaces the subscriber in full, so the profile travels unchanged
  // with the new description.
  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateSubscriber(
      accessToken,
      initialData.imsi,
      initialData.profileName,
      values.description,
    );
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Description"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update subscriber"
      submitLabel="Update"
      submittingLabel="Updating..."
    >
      <TextControl<FormValues>
        name="description"
        label="Description"
        helperText={`A note to identify this subscriber, up to ${MAX_DESCRIPTION_LENGTH} characters. Leave blank to remove it.`}
        multiline
        minRows={1}
        maxRows={3}
        autoFocus
      />
    </FormDialog>
  );
};

export default EditSubscriberDescriptionModal;
