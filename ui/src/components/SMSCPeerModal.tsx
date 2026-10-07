// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  DEFAULT_SMSC_PORT,
  createSMSCPeer,
  updateSMSCPeer,
  type SMSCPeer,
} from "@/queries/operator";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import NumberControl from "@/components/form/NumberControl";
import { isHostAddress } from "@/utils/ip";
import { msisdnRegex } from "@/components/subscriberIdentity";
import { PRODUCT } from "@/utils/product";

interface SMSCPeerModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  peer?: SMSCPeer;
}

const MAX_SERVICE_CENTRES = 16;

export const parseServiceCentres = (value: string): string[] =>
  value
    .split(/[\s,]+/)
    .map((v) => v.trim())
    .filter(Boolean);

const schema = yup.object({
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
  serviceCentres: yup
    .string()
    .default("")
    .test(
      "required",
      "At least one number is required",
      (v) => parseServiceCentres(v ?? "").length > 0,
    )
    .test(
      "max",
      `At most ${MAX_SERVICE_CENTRES} numbers`,
      (v) => parseServiceCentres(v ?? "").length <= MAX_SERVICE_CENTRES,
    )
    .test(
      "format",
      "Numbers must be E.164 numbers, for example +15550000000",
      (v) => parseServiceCentres(v ?? "").every((n) => msisdnRegex.test(n)),
    )
    .test("unique", "A number is listed twice", (v) => {
      const numbers = parseServiceCentres(v ?? "");
      return new Set(numbers).size === numbers.length;
    }),
});

type FormValues = yup.InferType<typeof schema>;

const SMSCPeerModal: React.FC<SMSCPeerModalProps> = ({
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
      address: peer?.address ?? "",
      port: peer?.port ?? DEFAULT_SMSC_PORT,
      serviceCentres: (peer?.serviceCentres ?? []).join(", "),
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    const input = {
      address: values.address.trim(),
      port: values.port,
      serviceCentres: parseServiceCentres(values.serviceCentres),
    };
    if (peer) {
      await updateSMSCPeer(accessToken, peer.id, input);
    } else {
      await createSMSCPeer(accessToken, input);
    }
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title={peer ? "Edit Service Center" : "Add Service Center"}
      description={`${PRODUCT.name} connects to this service center (SMSC) over Diameter. Messages phones send to one of its numbers go to this service center.`}
      form={form}
      onSubmit={submit}
      errorPrefix={
        peer
          ? "Failed to update service center"
          : "Failed to add service center"
      }
      submitLabel={peer ? "Update" : "Add"}
      submittingLabel={peer ? "Updating..." : "Adding..."}
      fullWidth={false}
    >
      <TextControl<FormValues>
        name="address"
        label="Address"
        placeholder="192.0.2.10"
        helperText="IP address of the service center's Diameter endpoint. Each service center needs its own address."
        autoFocus
      />
      <TextControl<FormValues>
        name="serviceCentres"
        label="Numbers"
        placeholder="+15550000000"
        helperText="E.164 numbers phones use to reach this service center, set as the SMSC number on their SIM. Separate with commas."
      />
      <NumberControl<FormValues>
        name="port"
        label="Port"
        min={1}
        max={65535}
        helperText={`SCTP port of the service center's Diameter endpoint (default ${DEFAULT_SMSC_PORT}).`}
      />
    </FormDialog>
  );
};

export default SMSCPeerModal;
