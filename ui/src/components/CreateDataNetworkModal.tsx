// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { FormControlLabel, Switch } from "@mui/material";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { createDataNetwork } from "@/queries/data_networks";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  DataNetworkFields,
  dataNetworkNameRegex,
  poolAndDnsSchema,
} from "@/components/dataNetworkForm";
import { IMS_DATA_NETWORK } from "@/utils/voice";

interface CreateDataNetworkModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export const schema = yup.object({
  name: yup
    .string()
    .matches(
      dataNetworkNameRegex,
      "Must be a valid name (e.g., internet, ims, core.mycompany)",
    )
    .required("Data Network Name is required"),
  ...poolAndDnsSchema,
});

type FormValues = yup.InferType<typeof schema>;

const CreateDataNetworkModal: React.FC<CreateDataNetworkModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();
  const [voice, setVoice] = useState(false);

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: {
      name: "",
      ipv4_pool: "10.45.0.0/22",
      ipv6_pool: "",
      dns: "8.8.8.8",
      mtu: 1456,
    },
  });

  const toggleVoice = (_: React.ChangeEvent<HTMLInputElement>, on: boolean) => {
    setVoice(on);
    form.setValue("name", on ? IMS_DATA_NETWORK : "", { shouldValidate: on });
  };

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await createDataNetwork(
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
      title="Create Data Network"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to create data network"
      submitLabel="Create"
      submittingLabel="Creating..."
      fullWidth={false}
    >
      <FormControlLabel
        control={<Switch checked={voice} onChange={toggleVoice} />}
        label="Voice (IMS)"
      />
      <TextControl<FormValues>
        name="name"
        label="Name"
        autoFocus
        disabled={voice}
      />
      <DataNetworkFields />
    </FormDialog>
  );
};

export default CreateDataNetworkModal;
