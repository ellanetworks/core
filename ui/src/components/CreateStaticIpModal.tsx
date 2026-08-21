// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Alert } from "@mui/material";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { useAuth } from "@/contexts/AuthContext";
import { createStaticIp, updateStaticIp } from "@/queries/data_networks";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import SubscriberSelectField from "@/components/SubscriberSelectField";

interface StaticIpEdit {
  imsi: string;
  ipVersion: string;
  address: string;
  active?: boolean;
}

interface CreateStaticIpModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  dataNetwork: string;
  ipv4Pool?: string;
  ipv6Pool?: string;
  edit?: StaticIpEdit;
}

const schema = yup.object({
  imsi: yup.string().trim().required(),
  address: yup.string().trim().required(),
});

type FormValues = yup.InferType<typeof schema>;

const CreateStaticIpModal: React.FC<CreateStaticIpModalProps> = ({
  open,
  onClose,
  onSuccess,
  dataNetwork,
  ipv4Pool,
  ipv6Pool,
  edit,
}) => {
  const { accessToken } = useAuth();
  const isEdit = !!edit;

  const form = useForm<FormValues>({
    mode: "onChange",
    resolver: yupResolver(schema),
    values: { imsi: edit?.imsi ?? "", address: edit?.address ?? "" },
  });

  const poolHelp = isEdit
    ? edit?.ipVersion === "ipv6"
      ? `IPv6 pool: ${ipv6Pool ?? "—"}`
      : `IPv4 pool: ${ipv4Pool ?? "—"}`
    : [
        ipv4Pool && `IPv4 pool: ${ipv4Pool}`,
        ipv6Pool && `IPv6 pool: ${ipv6Pool}`,
      ]
        .filter(Boolean)
        .join(" · ");

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    if (isEdit && edit) {
      await updateStaticIp(
        accessToken,
        dataNetwork,
        edit.imsi,
        edit.ipVersion,
        values.address.trim(),
      );
    } else {
      await createStaticIp(
        accessToken,
        dataNetwork,
        values.imsi.trim(),
        values.address.trim(),
      );
    }
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title={isEdit ? "Edit Static IP" : "Add Static IP"}
      form={form}
      onSubmit={submit}
      submitLabel="Save"
      submittingLabel="Saving..."
      fullWidth
    >
      {isEdit && edit?.active && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          This subscriber has an active session. Saving a new address releases
          it, and the subscriber reconnects on the new address.
        </Alert>
      )}
      <SubscriberSelectField
        control={form.control}
        name="imsi"
        dataNetwork={dataNetwork}
        open={open}
        readOnly={isEdit}
      />
      <TextControl<FormValues>
        name="address"
        label="Address"
        helperText={poolHelp}
        placeholder="e.g., 10.45.0.10 or 2001:db8:1::"
        autoFocus={isEdit}
      />
    </FormDialog>
  );
};

export default CreateStaticIpModal;
