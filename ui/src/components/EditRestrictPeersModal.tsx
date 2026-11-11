// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useController, useForm, type Control } from "react-hook-form";
import { FormControlLabel, Switch } from "@mui/material";
import { updateIMSSettings } from "@/queries/integrations";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";

export const RESTRICT_PEERS_HELP =
  "On: only allowed IMS peers can connect. Off: any IMS node can connect.";

interface EditRestrictPeersModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    pcscfAddresses: string[];
    homeDomain: string;
    advertiseVoice: boolean;
    restrictPeers: boolean;
  };
}

type FormValues = { restrictPeers: boolean };

const RestrictPeersSwitch: React.FC<{
  control: Control<FormValues>;
}> = ({ control }) => {
  const { field } = useController({ control, name: "restrictPeers" });

  return (
    <FormControlLabel
      control={
        <Switch
          checked={field.value}
          onChange={(_, on) => field.onChange(on)}
        />
      }
      label="Restrict to allowed peers"
    />
  );
};

const EditRestrictPeersModal: React.FC<EditRestrictPeersModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();
  const form = useForm<FormValues>({
    values: { restrictPeers: initialData.restrictPeers },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateIMSSettings(accessToken, {
      pcscfAddresses: initialData.pcscfAddresses,
      homeDomain: initialData.homeDomain,
      advertiseVoice: initialData.advertiseVoice,
      restrictPeers: values.restrictPeers,
    });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Restrict to Allowed Peers"
      description={RESTRICT_PEERS_HELP}
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update IMS peer restriction"
      submitLabel="Update"
      submittingLabel="Updating..."
    >
      <RestrictPeersSwitch control={form.control} />
    </FormDialog>
  );
};

export default EditRestrictPeersModal;
