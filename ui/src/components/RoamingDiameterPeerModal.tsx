// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  DEFAULT_ROAMING_DIAMETER_PORT,
  createRoamingDiameterPeer,
  updateRoamingDiameterPeer,
  type RoamingDiameterPeer,
} from "@/queries/roaming";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import NumberControl from "@/components/form/NumberControl";
import {
  ORIGIN_HOST_HELP,
  ORIGIN_HOST_LABEL,
  fqdnRegex,
} from "@/components/originHost";
import { isHostAddress } from "@/utils/ip";

interface RoamingDiameterPeerModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  peer?: RoamingDiameterPeer;
}

const schema = yup.object({
  originHost: yup
    .string()
    .default("")
    .test(
      "origin-host",
      "Origin-Host must be a fully qualified domain name, for example hss.epc.mnc001.mcc001.3gppnetwork.org",
      (v) => !v?.trim() || fqdnRegex.test(v.trim()),
    ),
  address: yup
    .string()
    .default("")
    .test("required", "Address is required", (v) => !!v?.trim())
    .test(
      "address",
      "Address must be an IPv4 or IPv6 address",
      (v) => !v?.trim() || isHostAddress(v.trim()),
    ),
  port: yup
    .number()
    .typeError("Port must be a number")
    .integer("Port must be a whole number")
    .min(1, "Port must be between 1 and 65535")
    .max(65535, "Port must be between 1 and 65535")
    .required("Port is required"),
});

type FormValues = yup.InferType<typeof schema>;

const RoamingDiameterPeerModal: React.FC<RoamingDiameterPeerModalProps> = ({
  open,
  onClose,
  onSuccess,
  peer,
}) => {
  const { accessToken } = useAuth();
  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: {
      originHost: peer?.originHost ?? "",
      address: peer?.address ?? "",
      port: peer?.port ?? DEFAULT_ROAMING_DIAMETER_PORT,
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    const input = {
      originHost: values.originHost.trim(),
      address: values.address.trim(),
      port: values.port,
    };
    if (peer) {
      await updateRoamingDiameterPeer(accessToken, peer.id, input);
    } else {
      await createRoamingDiameterPeer(accessToken, input);
    }
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title={peer ? "Edit Diameter Peer" : "Add Diameter Peer"}
      form={form}
      onSubmit={submit}
      errorPrefix={
        peer ? "Failed to update Diameter peer" : "Failed to add Diameter peer"
      }
      submitLabel={peer ? "Update" : "Add"}
      submittingLabel={peer ? "Updating..." : "Adding..."}
    >
      <TextControl<FormValues>
        name="address"
        label="Address"
        placeholder="192.0.2.10"
        helperText="IP address of the peer's Diameter endpoint."
        autoFocus
      />
      <NumberControl<FormValues>
        name="port"
        label="Port"
        min={1}
        max={65535}
        helperText={`SCTP port of the peer's Diameter endpoint (default ${DEFAULT_ROAMING_DIAMETER_PORT}).`}
      />
      <TextControl<FormValues>
        name="originHost"
        label={ORIGIN_HOST_LABEL}
        placeholder="hss.epc.mnc001.mcc001.3gppnetwork.org"
        helperText={ORIGIN_HOST_HELP}
      />
    </FormDialog>
  );
};

export default RoamingDiameterPeerModal;
