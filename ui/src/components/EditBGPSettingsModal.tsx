// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateBGPSettings } from "@/queries/bgp";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";

interface EditBGPSettingsModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    enabled: boolean;
    localAS: string;
    routerID: string;
    listenAddress: string;
  };
}

const schema = yup.object({
  localAS: yup
    .string()
    .matches(/^\d+$/, "Local AS must be a number")
    .required("Local AS is required")
    .test("range", "Local AS must be between 1 and 4294967295", (val) => {
      const n = Number(val);
      return n >= 1 && n <= 4294967295;
    }),
  routerID: yup
    .string()
    .default("")
    .test("ipv4", "Router ID must be a valid IPv4 address or empty", (val) => {
      if (!val || val === "") return true;
      return /^(\d{1,3}\.){3}\d{1,3}$/.test(val);
    }),
  listenAddress: yup
    .string()
    .required("Listen address is required")
    .test(
      "host-port",
      "Listen address must be in host:port or :port format",
      (val) => {
        if (!val) return false;
        const match = val.match(/^(.*):(\d+)$/);
        if (!match) return false;
        const port = Number(match[2]);
        return port >= 1 && port <= 65535;
      },
    ),
});

type FormValues = yup.InferType<typeof schema>;

const EditBGPSettingsModal: React.FC<EditBGPSettingsModalProps> = ({
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
      localAS: initialData.localAS,
      routerID: initialData.routerID,
      listenAddress: initialData.listenAddress,
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateBGPSettings(accessToken, {
      enabled: initialData.enabled,
      localAS: Number(values.localAS),
      routerID: values.routerID,
      listenAddress: values.listenAddress,
    });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit BGP Settings"
      description="Configure the local BGP speaker. Changes may require the BGP speaker to restart."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update BGP settings"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <TextControl<FormValues> name="localAS" label="Local AS" autoFocus />
      <TextControl<FormValues> name="routerID" label="Router ID" />
      <TextControl<FormValues> name="listenAddress" label="Listen Address" />
    </FormDialog>
  );
};

export default EditBGPSettingsModal;
