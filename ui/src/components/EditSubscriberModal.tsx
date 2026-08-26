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
import ProfileSelectField, {
  useProfileNames,
} from "@/components/ProfileSelectField";

interface EditSubscriberModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    imsi: string;
    profileName: string;
    description: string;
  };
}

const schema = yup.object({
  imsi: yup.string().required(),
  profileName: yup.string().required("Profile is required."),
  description: yup
    .string()
    .default("")
    .max(
      MAX_DESCRIPTION_LENGTH,
      `Description must be at most ${MAX_DESCRIPTION_LENGTH} characters.`,
    ),
});

type FormValues = yup.InferType<typeof schema>;

const EditSubscriberModal: React.FC<EditSubscriberModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();
  const profilesQuery = useProfileNames(open);

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: {
      imsi: initialData.imsi,
      profileName: initialData.profileName,
      description: initialData.description,
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateSubscriber(
      accessToken,
      values.imsi,
      values.profileName,
      values.description,
    );
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Subscriber"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update subscriber"
      submitLabel="Update"
      submittingLabel="Updating..."
    >
      <TextControl<FormValues> name="imsi" label="IMSI" disabled />
      <ProfileSelectField
        control={form.control}
        name="profileName"
        profiles={profilesQuery.data ?? []}
      />
      <TextControl<FormValues>
        name="description"
        label="Description (optional)"
        helperText={`A note to identify this subscriber, up to ${MAX_DESCRIPTION_LENGTH} characters`}
        multiline
        minRows={2}
      />
    </FormDialog>
  );
};

export default EditSubscriberModal;
