// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useController, useForm, type Control } from "react-hook-form";
import { FormControlLabel, Switch } from "@mui/material";
import { updateIMSSettings } from "@/queries/integrations";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";

export const ADVERTISE_VOICE_HELP =
  "Whether Ella Core tells phones, when they attach or register on 4G and 5G, that IMS voice is supported. On: every phone is told, whatever the IMS configuration; some phones leave 4G or 5G without it. Off: no phone is told, even when the IMS is configured.";

interface EditAdvertiseVoiceModalProps {
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

type FormValues = { advertiseVoice: boolean };

const AdvertiseVoiceSwitch: React.FC<{
  control: Control<FormValues>;
}> = ({ control }) => {
  const { field } = useController({ control, name: "advertiseVoice" });

  return (
    <FormControlLabel
      control={
        <Switch
          checked={field.value}
          onChange={(_, on) => field.onChange(on)}
        />
      }
      label="Advertise IMS voice"
    />
  );
};

const EditAdvertiseVoiceModal: React.FC<EditAdvertiseVoiceModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();
  const form = useForm<FormValues>({
    values: { advertiseVoice: initialData.advertiseVoice },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateIMSSettings(accessToken, {
      pcscfAddresses: initialData.pcscfAddresses,
      homeDomain: initialData.homeDomain,
      advertiseVoice: values.advertiseVoice,
      restrictPeers: initialData.restrictPeers,
    });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Advertise IMS Voice"
      description={ADVERTISE_VOICE_HELP}
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update IMS voice advertisement"
      submitLabel="Update"
      submittingLabel="Updating..."
    >
      <AdvertiseVoiceSwitch control={form.control} />
    </FormDialog>
  );
};

export default EditAdvertiseVoiceModal;
