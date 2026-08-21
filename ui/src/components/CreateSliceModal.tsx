// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { createSlice } from "@/queries/slices";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  SliceIdentityFields,
  sliceIdentitySchema,
} from "@/components/sliceForm";

interface CreateSliceModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object({
  name: yup
    .string()
    .max(255, "Name must be 255 characters or less")
    .required("Name is required"),
  ...sliceIdentitySchema,
});

type FormValues = yup.InferType<typeof schema>;

const CreateSliceModal: React.FC<CreateSliceModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: { name: "", sst: 1, sd: "" },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return;
    await createSlice(accessToken, values.name, values.sst, values.sd);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Create Network Slice"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to create network slice"
      submitLabel="Create"
      submittingLabel="Creating..."
      fullWidth={false}
    >
      <TextControl<FormValues> name="name" label="Name" autoFocus />
      <SliceIdentityFields />
    </FormDialog>
  );
};

export default CreateSliceModal;
