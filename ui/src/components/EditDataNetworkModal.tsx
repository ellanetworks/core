// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateDataNetwork, APIDataNetwork } from "@/queries/data_networks";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  DataNetworkFields,
  poolAndDnsSchema,
} from "@/components/dataNetworkForm";

interface EditDataNetworkModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APIDataNetwork;
}

export const schema = yup.object({
  name: yup.string().required(),
  ...poolAndDnsSchema,
});

type FormValues = yup.InferType<typeof schema>;

const EditDataNetworkModal: React.FC<EditDataNetworkModalProps> = ({
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
      ipv4_pool: initialData.ipv4_pool,
      ipv6_pool: initialData.ipv6_pool || "",
      dns: initialData.dns,
      mtu: initialData.mtu,
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return;
    await updateDataNetwork(
      accessToken,
      values.name,
      values.ipv4_pool,
      values.dns,
      values.mtu,
      values.ipv6_pool || undefined,
    );
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Data Network"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update data network"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <TextControl<FormValues> name="name" label="Name" disabled />
      <DataNetworkFields autoFocusPool />
    </FormDialog>
  );
};

export default EditDataNetworkModal;
