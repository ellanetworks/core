// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { Alert, Box } from "@mui/material";
import { updateIMSSettings } from "@/queries/integrations";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import { isHostName } from "@/utils/ip";

export const HOME_DOMAIN_HELP =
  "SIP domain of your subscribers' public identities. Must match your IMS and the domain on ISIMs. Leave empty to use the default.";

interface EditIMSHomeDomainModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    homeDomain: string;
    derivedHomeDomain: string;
    pcscfAddresses: string[];
    advertiseVoice: boolean;
    restrictPeers: boolean;
  };
}

export const isValidHomeDomain = (value: string) =>
  value.length <= 253 && value.includes(".") && isHostName(value);

const schema = yup.object({
  homeDomain: yup
    .string()
    .default("")
    .test(
      "domain",
      "Must be a domain name with at least two labels, for example ims.example.com",
      (v) => !v?.trim() || isValidHomeDomain(v.trim()),
    ),
});

type FormValues = yup.InferType<typeof schema>;

const normalize = (value: string | undefined) =>
  (value ?? "").trim().toLowerCase();

const EditIMSHomeDomainModal: React.FC<EditIMSHomeDomainModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();
  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: { homeDomain: initialData.homeDomain },
  });
  const current = normalize(
    useWatch({ control: form.control, name: "homeDomain" }),
  );
  const [confirmedValue, setConfirmedValue] = useState<string | null>(null);
  const confirming = confirmedValue !== null && confirmedValue === current;
  const changed = current !== normalize(initialData.homeDomain);

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    const homeDomain = normalize(values.homeDomain);
    if (homeDomain === normalize(initialData.homeDomain)) return;
    if (confirmedValue !== homeDomain) {
      setConfirmedValue(homeDomain);
      return false;
    }
    await updateIMSSettings(accessToken, {
      pcscfAddresses: initialData.pcscfAddresses,
      homeDomain,
      advertiseVoice: initialData.advertiseVoice,
      restrictPeers: initialData.restrictPeers,
    });
  };

  const newDomain = current || initialData.derivedHomeDomain;

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Home Domain"
      description={HOME_DOMAIN_HELP}
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update the home domain"
      submitLabel={confirming ? "Confirm" : "Update"}
      submittingLabel="Updating..."
    >
      <TextControl<FormValues>
        name="homeDomain"
        label="Home Domain"
        placeholder={initialData.derivedHomeDomain}
        autoFocus
      />
      {confirming && changed && (
        <Alert severity="warning" sx={{ mt: 2 }}>
          {`Changing the home domain to ${newDomain}:`}
          <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
            <li>
              Rewrites the public service identities of application servers to
              the new domain.
            </li>
            <li>
              Deregisters registered IMS users. They register again with their
              new identities.
            </li>
            <li>Your IMS must be configured with the same domain.</li>
          </Box>
        </Alert>
      )}
    </FormDialog>
  );
};

export default EditIMSHomeDomainModal;
