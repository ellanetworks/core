// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Box, Checkbox, FormControlLabel } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { useController, useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  updatePolicy,
  getPolicy,
  type APIPolicy,
  type PolicyRules,
} from "@/queries/policies";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  PolicyFields,
  policySchema,
  splitAmbr,
  useNetworkOptions,
} from "@/components/policyForm";

interface EditPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APIPolicy;
}

const schema = yup.object({
  name: yup.string().required(),
  ...policySchema,
});

type FormValues = yup.InferType<typeof schema>;

const EditPolicyModal: React.FC<EditPolicyModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken, authReady } = useAuth();
  const { dataNetworks, slices } = useNetworkOptions(open);

  const policyQuery = useQuery({
    queryKey: ["policy", initialData.name],
    queryFn: () => getPolicy(accessToken!, initialData.name),
    enabled: open && authReady && !!accessToken,
  });

  const policy = policyQuery.data;
  const up = splitAmbr(policy?.session_ambr_uplink ?? "");
  const down = splitAmbr(policy?.session_ambr_downlink ?? "");
  const currentRules: PolicyRules | undefined = policy?.rules;

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: {
      name: policy?.name ?? initialData.name,
      sliceName: policy?.slice_name ?? initialData.slice_name,
      ambrUpValue: up.num,
      ambrUpUnit: up.unit,
      ambrDownValue: down.num,
      ambrDownUnit: down.unit,
      fiveQi: policy?.var5qi ?? initialData.var5qi,
      arp: policy?.arp ?? initialData.arp,
      dataNetworkName:
        policy?.data_network_name ?? initialData.data_network_name,
      isDefault: policy?.default ?? initialData.default,
    },
  });

  const { field: isDefaultField } = useController({
    control: form.control,
    name: "isDefault",
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updatePolicy(accessToken, values.name, {
      profile_name: initialData.profile_name,
      slice_name: values.sliceName,
      data_network_name: values.dataNetworkName,
      session_ambr_uplink: `${values.ambrUpValue} ${values.ambrUpUnit}`,
      session_ambr_downlink: `${values.ambrDownValue} ${values.ambrDownUnit}`,
      var5qi: values.fiveQi,
      arp: values.arp,
      rules: currentRules,
      default: values.isDefault,
    });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Policy"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update policy"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <TextControl<FormValues> name="name" label="Name" disabled />
      <PolicyFields<FormValues>
        control={form.control}
        dataNetworks={dataNetworks ?? []}
        slices={slices ?? []}
      />
      <Box sx={{ mt: 1 }}>
        <FormControlLabel
          control={
            <Checkbox
              checked={!!isDefaultField.value}
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

export default EditPolicyModal;
