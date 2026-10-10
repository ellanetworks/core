// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { ToggleButton, ToggleButtonGroup, Typography } from "@mui/material";
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
  voiceAvailable?: boolean;
}

type Kind = "data" | "voice";

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
  voiceAvailable = false,
}) => {
  const { accessToken } = useAuth();
  const [kind, setKind] = useState<Kind>("data");

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

  const chooseKind = (_: React.MouseEvent<HTMLElement>, next: Kind | null) => {
    if (!next || next === kind) return;

    setKind(next);
    form.setValue("name", next === "voice" ? IMS_DATA_NETWORK : "", {
      shouldValidate: next === "voice",
    });
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
      {voiceAvailable && (
        <>
          <ToggleButtonGroup
            value={kind}
            exclusive
            onChange={chooseKind}
            size="small"
            sx={{ mt: 1, mb: 1 }}
            aria-label="Data network type"
          >
            <ToggleButton value="data">Data</ToggleButton>
            <ToggleButton value="voice">Voice (IMS)</ToggleButton>
          </ToggleButtonGroup>
          {kind === "voice" && (
            <Typography variant="body2" color="textSecondary" sx={{ mb: 1 }}>
              Handsets reach IMS on the data network named &quot;
              {IMS_DATA_NETWORK}&quot; for voice and video calls.
            </Typography>
          )}
        </>
      )}
      <TextControl<FormValues>
        name="name"
        label="Name"
        autoFocus={kind === "data"}
        slotProps={{ input: { readOnly: kind === "voice" } }}
      />
      <DataNetworkFields />
    </FormDialog>
  );
};

export default CreateDataNetworkModal;
