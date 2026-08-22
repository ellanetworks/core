// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { createBGPPeer, type RejectedPrefix } from "@/queries/bgp";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  BGPPeerIdentityFields,
  ImportPrefixEditor,
  RejectedPrefixesPanel,
  bgpPeerSchema,
} from "@/components/bgpPeerForm";

interface CreateBGPPeerModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  rejectedPrefixes?: RejectedPrefix[];
}

const schema = yup.object(bgpPeerSchema);

type FormValues = yup.InferType<typeof schema>;

const CreateBGPPeerModal: React.FC<CreateBGPPeerModalProps> = ({
  open,
  onClose,
  onSuccess,
  rejectedPrefixes = [],
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: {
      address: "",
      remoteAS: 64512,
      holdTime: 90,
      password: "",
      description: "",
      importPrefixes: [],
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await createBGPPeer(accessToken, {
      address: values.address,
      remoteAS: values.remoteAS,
      holdTime: values.holdTime,
      password: values.password || undefined,
      description: values.description || undefined,
      importPrefixes: values.importPrefixes,
    });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Create BGP Peer"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to create BGP peer"
      submitLabel="Create"
      submittingLabel="Creating..."
    >
      <BGPPeerIdentityFields />
      <TextControl<FormValues>
        name="password"
        label="Password"
        type="password"
        helperText="TCP MD5 authentication password (optional)"
      />
      <TextControl<FormValues> name="description" label="Description" />
      <ImportPrefixEditor control={form.control} name="importPrefixes" />
      <RejectedPrefixesPanel rejectedPrefixes={rejectedPrefixes} />
    </FormDialog>
  );
};

export default CreateBGPPeerModal;
