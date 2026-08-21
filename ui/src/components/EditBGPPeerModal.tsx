// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { Button, Stack, TextField } from "@mui/material";
import { useController, useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  updateBGPPeer,
  type BGPPeer,
  type RejectedPrefix,
} from "@/queries/bgp";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  BGPPeerIdentityFields,
  ImportPrefixEditor,
  RejectedPrefixesPanel,
  bgpPeerSchema,
} from "@/components/bgpPeerForm";

interface EditBGPPeerModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  peer: BGPPeer;
  rejectedPrefixes?: RejectedPrefix[];
}

const schema = yup.object(bgpPeerSchema);

type FormValues = yup.InferType<typeof schema>;

const EditBGPPeerModal: React.FC<EditBGPPeerModalProps> = ({
  open,
  onClose,
  onSuccess,
  peer,
  rejectedPrefixes = [],
}) => {
  const { accessToken } = useAuth();
  const [clearPassword, setClearPassword] = useState(false);

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: {
      address: peer.address,
      remoteAS: peer.remoteAS,
      holdTime: peer.holdTime,
      password: "",
      description: peer.description,
      importPrefixes: peer.importPrefixes ?? [],
    },
  });

  const { field: passwordField } = useController({
    control: form.control,
    name: "password",
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;

    let password: string | undefined;
    if (clearPassword) {
      password = "";
    } else if (values.password) {
      password = values.password;
    }

    await updateBGPPeer(accessToken, peer.id, {
      address: values.address,
      remoteAS: values.remoteAS,
      holdTime: values.holdTime,
      password,
      description: values.description || undefined,
      importPrefixes: values.importPrefixes,
    });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit BGP Peer"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update BGP peer"
      submitLabel="Update"
      submittingLabel="Updating..."
    >
      <BGPPeerIdentityFields />
      <Stack direction="row" spacing={1} sx={{ alignItems: "flex-start" }}>
        <TextField
          fullWidth
          label="Password"
          type="password"
          value={passwordField.value ?? ""}
          onChange={passwordField.onChange}
          onBlur={passwordField.onBlur}
          inputRef={passwordField.ref}
          margin="normal"
          disabled={clearPassword}
          placeholder={
            peer.hasPassword
              ? "Leave empty to keep current password"
              : "Optional"
          }
          helperText={
            clearPassword
              ? "Password will be removed on save"
              : "TCP MD5 authentication password"
          }
        />
        {peer.hasPassword && (
          <Button
            size="small"
            variant="outlined"
            color={clearPassword ? "primary" : "error"}
            onClick={() => {
              setClearPassword((value) => !value);
              if (!clearPassword) form.setValue("password", "");
            }}
            sx={{ mt: "16px", minWidth: 70, height: 56 }}
          >
            {clearPassword ? "Undo" : "Clear"}
          </Button>
        )}
      </Stack>
      <TextControl<FormValues> name="description" label="Description" />
      <ImportPrefixEditor control={form.control} name="importPrefixes" />
      <RejectedPrefixesPanel rejectedPrefixes={rejectedPrefixes} />
    </FormDialog>
  );
};

export default EditBGPPeerModal;
