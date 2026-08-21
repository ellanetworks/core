// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect } from "react";
import { Alert, Box, Checkbox, FormControlLabel } from "@mui/material";
import { useController, useForm, useWatch } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { createPolicy } from "@/queries/policies";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  PolicyFields,
  policySchema,
  useNetworkOptions,
} from "@/components/policyForm";

interface CreatePolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  profileName: string;
  policyCount: number;
}

const schema = yup.object({
  name: yup.string().min(1).max(256).required("Name is required"),
  ...policySchema,
});

type FormValues = yup.InferType<typeof schema>;

const CreatePolicyModal: React.FC<CreatePolicyModalProps> = ({
  open,
  onClose,
  onSuccess,
  profileName,
  policyCount,
}) => {
  const { accessToken } = useAuth();
  const isFirstPolicy = policyCount === 0;
  const { dataNetworks, slices, isError } = useNetworkOptions(open);

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: {
      name: "",
      sliceName: "",
      ambrUpValue: 100,
      ambrUpUnit: "Mbps",
      ambrDownValue: 100,
      ambrDownUnit: "Mbps",
      fiveQi: 9,
      arp: 1,
      dataNetworkName: "",
      isDefault: false,
    },
  });

  const { field: isDefaultField } = useController({
    control: form.control,
    name: "isDefault",
  });
  const sliceName = useWatch({ control: form.control, name: "sliceName" });
  const dataNetworkName = useWatch({
    control: form.control,
    name: "dataNetworkName",
  });

  useEffect(() => {
    if (!slices?.length) return;
    if (sliceName && slices.includes(sliceName)) return;
    form.setValue("sliceName", slices[0], { shouldValidate: true });
  }, [slices, sliceName, form]);

  useEffect(() => {
    if (!dataNetworks?.length) return;
    if (dataNetworkName && dataNetworks.includes(dataNetworkName)) return;
    form.setValue("dataNetworkName", dataNetworks[0], { shouldValidate: true });
  }, [dataNetworks, dataNetworkName, form]);

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await createPolicy(accessToken, {
      name: values.name,
      profile_name: profileName,
      slice_name: values.sliceName,
      data_network_name: values.dataNetworkName,
      session_ambr_uplink: `${values.ambrUpValue} ${values.ambrUpUnit}`,
      session_ambr_downlink: `${values.ambrDownValue} ${values.ambrDownUnit}`,
      var5qi: values.fiveQi,
      arp: values.arp,
      default: isFirstPolicy || values.isDefault,
    });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Create Policy"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to create policy"
      submitLabel="Create"
      submittingLabel="Creating..."
      fullWidth={false}
    >
      {isError && (
        <Alert severity="error" sx={{ mb: 2 }}>
          Failed to load dropdown data. Please close and try again.
        </Alert>
      )}
      <TextControl<FormValues> name="name" label="Name" autoFocus />
      <PolicyFields<FormValues>
        control={form.control}
        dataNetworks={dataNetworks ?? []}
        slices={slices ?? []}
      />
      <Box sx={{ mt: 1 }}>
        <FormControlLabel
          control={
            <Checkbox
              checked={isFirstPolicy || !!isDefaultField.value}
              disabled={isFirstPolicy}
              onChange={(event) =>
                isDefaultField.onChange(event.target.checked)
              }
            />
          }
          label="Use this policy when a 4G subscriber attaches without requesting an APN"
        />
      </Box>
    </FormDialog>
  );
};

export default CreatePolicyModal;
