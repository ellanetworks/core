// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { createProfile } from "@/queries/profiles";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  AccessCheckboxes,
  AmbrFields,
  ambrSchema,
} from "@/components/profileForm";

interface CreateProfileModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object({
  name: yup.string().min(1).max(255).required("Name is required"),
  ...ambrSchema,
  allow4g: yup.boolean().required(),
  allow5g: yup.boolean().required(),
});

type FormValues = yup.InferType<typeof schema>;

const CreateProfileModal: React.FC<CreateProfileModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: {
      name: "",
      ambrUpValue: 100,
      ambrUpUnit: "Mbps",
      ambrDownValue: 100,
      ambrDownUnit: "Mbps",
      allow4g: true,
      allow5g: true,
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await createProfile(
      accessToken,
      values.name,
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
      title="Create Profile"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to create profile"
      submitLabel="Create"
      submittingLabel="Creating..."
      fullWidth={false}
    >
      <TextControl<FormValues> name="name" label="Name" autoFocus />
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

export default CreateProfileModal;
