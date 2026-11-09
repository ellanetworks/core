// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { useQuery } from "@tanstack/react-query";
import { Alert, Link as MuiLink } from "@mui/material";
import { Link as RouterLink } from "react-router-dom";
import { updateOperatorID } from "@/queries/operator";
import { getIMSSettings, type IMSSettings } from "@/queries/integrations";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import { PRODUCT } from "@/utils/product";

interface EditOperatorIdModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    mcc: string;
    mnc: string;
  };
}

const schema = yup.object({
  mcc: yup
    .string()
    .matches(/^\d{3}$/, "MCC must be a 3 decimal digit")
    .required("MCC is required"),
  mnc: yup
    .string()
    .matches(/^\d{2,3}$/, "MNC must be a 2 or 3 decimal digit")
    .required("MNC is required"),
});

type FormValues = yup.InferType<typeof schema>;

const IntegrationsLink: React.FC<{
  tab: string;
  label: string;
  onClick: () => void;
}> = ({ tab, label, onClick }) => (
  <MuiLink
    component={RouterLink}
    to={`/integrations/${tab}`}
    onClick={onClick}
    sx={{ color: (theme) => theme.palette.link }}
  >
    {label}
  </MuiLink>
);

const EditOperatorIdModal: React.FC<EditOperatorIdModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken, authReady } = useAuth();
  const imsQuery = useQuery<IMSSettings>({
    queryKey: ["integrations-ims"],
    enabled: open && authReady && !!accessToken,
    queryFn: () => getIMSSettings(accessToken!),
  });
  const homeDomainConfigured = !!imsQuery.data?.homeDomain;

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: { mcc: initialData.mcc, mnc: initialData.mnc },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateOperatorID(accessToken, values.mcc, values.mnc);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Operator ID"
      description="The Operator ID is a combination of Mobile Country Code (MCC) and Mobile Network Code (MNC). The Operator ID is used to uniquely identify the operator in the network."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update operator ID"
      submitLabel="Update"
      submittingLabel="Updating..."
    >
      <Alert severity="warning" sx={{ mb: 1 }}>
        Changing the MCC or MNC changes {PRODUCT.name}&apos;s Diameter realm and
        Origin-Host. Update them in your IMS, your SMSCs and your roaming
        partners&apos; Diameter edge agents or IPX, and replace the SEPP
        certificate: its FQDN is under the old MCC and MNC. The new values are
        shown in Integrations ›{" "}
        <IntegrationsLink tab="ims" label="IMS" onClick={onClose} />,{" "}
        <IntegrationsLink tab="sms" label="SMS" onClick={onClose} /> and{" "}
        <IntegrationsLink tab="roaming" label="Roaming" onClick={onClose} />.
        {homeDomainConfigured &&
          " The IMS home domain is configured separately and will not change."}
      </Alert>
      <TextControl<FormValues> name="mcc" label="MCC" autoFocus />
      <TextControl<FormValues> name="mnc" label="MNC" />
    </FormDialog>
  );
};

export default EditOperatorIdModal;
