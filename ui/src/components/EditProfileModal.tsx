// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { APIProfile, updateProfile } from "@/queries/profiles";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import { AccessCheckboxes, parseAmbr } from "@/components/profileForm";
import { AmbrFields, ambrSchema } from "@/components/form/BitrateFields";

interface EditProfileModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APIProfile;
}

const schema = yup.object({
  name: yup.string().required(),
  ...ambrSchema,
  allow4g: yup.boolean().required(),
  allow5g: yup.boolean().required(),
});

type FormValues = yup.InferType<typeof schema>;

const EditProfileModal: React.FC<EditProfileModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();

  const up = parseAmbr(initialData.ue_ambr_uplink);
  const down = parseAmbr(initialData.ue_ambr_downlink);

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: {
      name: initialData.name,
      ambrUpValue: up.num,
      ambrUpUnit: up.unit,
      ambrDownValue: down.num,
      ambrDownUnit: down.unit,
      allow4g: initialData.allow_4g,
      allow5g: initialData.allow_5g,
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateProfile(
      accessToken,
      initialData.name,
      `${values.ambrUpValue} ${values.ambrUpUnit}`,
      `${values.ambrDownValue} ${values.ambrDownUnit}`,
      values.allow4g,
      values.allow5g,
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
      errorPrefix="Failed to update profile"
      submitLabel="Save"
      submittingLabel="Saving..."
      fullWidth={false}
    >
      <TextControl<FormValues> name="name" label="Name" disabled />
      <AmbrFields<FormValues>
        valueName="ambrUpValue"
        unitName="ambrUpUnit"
        label="Bitrate Uplink"
      />
      <AmbrFields<FormValues>
        valueName="ambrDownValue"
        unitName="ambrDownUnit"
        label="Bitrate Downlink"
      />
      <AccessCheckboxes
        control={form.control}
        allow4gName="allow4g"
        allow5gName="allow5g"
      />
    </FormDialog>
  );
};

export default EditProfileModal;
