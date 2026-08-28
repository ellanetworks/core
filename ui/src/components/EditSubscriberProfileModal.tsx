// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { updateSubscriber } from "@/queries/subscribers";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import ProfileSelectField, {
  useProfileNames,
} from "@/components/ProfileSelectField";
import type { EditSubscriberFields } from "@/components/subscriberIdentity";

interface EditSubscriberProfileModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: EditSubscriberFields;
}

interface FormValues {
  profileName: string;
}

const EditSubscriberProfileModal: React.FC<EditSubscriberProfileModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();
  const profilesQuery = useProfileNames(open);

  const form = useForm<FormValues>({
    values: { profileName: initialData.profileName },
  });

  // The API replaces the subscriber in full, so the description travels
  // unchanged with the new profile.
  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateSubscriber(
      accessToken,
      initialData.imsi,
      values.profileName,
      initialData.description,
    );
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Profile"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update subscriber"
      submitLabel="Update"
      submittingLabel="Updating..."
    >
      <ProfileSelectField
        control={form.control}
        name="profileName"
        profiles={profilesQuery.data ?? []}
      />
    </FormDialog>
  );
};

export default EditSubscriberProfileModal;
