// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateSlice, type APISlice } from "@/queries/slices";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  SliceIdentityFields,
  sliceIdentitySchema,
} from "@/components/sliceForm";

interface EditSliceModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APISlice;
}

const schema = yup.object({
  name: yup.string().required(),
  ...sliceIdentitySchema,
});

type FormValues = yup.InferType<typeof schema>;

const EditSliceModal: React.FC<EditSliceModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: {
      name: initialData.name,
      sst: initialData.sst,
      sd: initialData.sd,
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return;
    await updateSlice(accessToken, values.name, values.sst, values.sd);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Network Slice"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update network slice"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <TextControl<FormValues> name="name" label="Name" disabled />
      <SliceIdentityFields autoFocusSst />
    </FormDialog>
  );
};

export default EditSliceModal;
