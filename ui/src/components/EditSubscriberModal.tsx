// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { updateSubscriber } from "@/queries/subscribers";
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
  };
}

interface FormValues {
  imsi: string;
  profileName: string;
}

const EditSubscriberModal: React.FC<EditSubscriberModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();
  const profilesQuery = useProfileNames(open);

  const form = useForm<FormValues>({
    values: {
      imsi: initialData.imsi,
      profileName: initialData.profileName,
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateSubscriber(accessToken, values.imsi, values.profileName);
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
      fullWidth={false}
    >
      <TextControl<FormValues> name="imsi" label="IMSI" disabled />
      <ProfileSelectField
        control={form.control}
        name="profileName"
        profiles={profilesQuery.data ?? []}
      />
    </FormDialog>
  );
};

export default EditSubscriberModal;
