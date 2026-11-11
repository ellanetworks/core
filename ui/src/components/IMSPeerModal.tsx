// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  createIMSPeer,
  updateIMSPeer,
  type IMSPeer,
} from "@/queries/integrations";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  ORIGIN_HOST_HELP,
  ORIGIN_HOST_LABEL,
  fqdnRegex,
} from "@/components/originHost";
import { addressFamily, isValidCidr } from "@/utils/ip";

interface IMSPeerModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  peer?: IMSPeer;
  initialValues?: { name?: string; originHost?: string; addresses?: string[] };
}

export const splitSourceAddresses = (value: string | undefined) =>
  (value ?? "")
    .split(/[\s,]+/)
    .map((v) => v.trim())
    .filter(Boolean);

const isSourceAddress = (value: string) =>
  isValidCidr(value) || (addressFamily(value) !== null && !value.includes("%"));

export const toCIDR = (value: string) => {
  if (value.includes("/")) return value;
  return addressFamily(value) === 6 ? `${value}/128` : `${value}/32`;
};

const fqdn = (field: string, example: string) =>
  yup
    .string()
    .default("")
    .test(
      "fqdn",
      `${field} must be a fully qualified domain name, for example ${example}`,
      (v) => !v?.trim() || fqdnRegex.test(v.trim()),
    );

const schema = yup.object({
  name: yup
    .string()
    .default("")
    .test("required", "Name is required", (v) => !!v?.trim())
    .max(255, "Name must be at most 255 characters")
    .test(
      "dots",
      "Name cannot be . or ..",
      (v) => v?.trim() !== "." && v?.trim() !== "..",
    ),
  originHost: fqdn(
    ORIGIN_HOST_LABEL,
    "scscf.ims.mnc001.mcc001.3gppnetwork.org",
  ),
  realm: fqdn("Realm", "ims.mnc001.mcc001.3gppnetwork.org"),
  addresses: yup
    .string()
    .default("")
    .test("format", function (value) {
      const addresses = splitSourceAddresses(value);
      if (addresses.length === 0) {
        return this.createError({
          message: "At least one source address is required",
        });
      }
      const invalid = addresses.find((a) => !isSourceAddress(a));
      if (invalid) {
        return this.createError({
          message: `${invalid} is not an IP address or CIDR, for example 10.0.0.0/24`,
        });
      }
      const normalized = addresses.map((a) => toCIDR(a).toLowerCase());
      const duplicate = addresses.find(
        (_, i) => normalized.indexOf(normalized[i]) !== i,
      );
      if (duplicate) {
        return this.createError({
          message: `${duplicate} is listed more than once`,
        });
      }
      return true;
    }),
});

type FormValues = yup.InferType<typeof schema>;

const IMSPeerModal: React.FC<IMSPeerModalProps> = ({
  open,
  onClose,
  onSuccess,
  peer,
  initialValues,
}) => {
  const { accessToken } = useAuth();
  const editing = !!peer;
  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: {
      name: peer?.name ?? initialValues?.name ?? "",
      originHost: peer?.originHost ?? initialValues?.originHost ?? "",
      realm: peer?.realm ?? "",
      addresses: (peer?.addresses ?? initialValues?.addresses ?? []).join(", "),
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    const input = {
      originHost: values.originHost.trim(),
      realm: values.realm.trim(),
      addresses: splitSourceAddresses(values.addresses).map(toCIDR),
    };
    if (peer) {
      await updateIMSPeer(accessToken, peer.name, input);
    } else {
      await createIMSPeer(accessToken, { name: values.name.trim(), ...input });
    }
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title={editing ? "Edit Allowed Peer" : "Add Allowed Peer"}
      description="An IMS node allowed to connect to Ella Core. A peer connecting from one of the source addresses is admitted when its Origin-Host and realm, if set here, also match."
      form={form}
      onSubmit={submit}
      errorPrefix={
        editing ? "Failed to update allowed peer" : "Failed to add allowed peer"
      }
      submitLabel={editing ? "Update" : "Add"}
      submittingLabel={editing ? "Updating..." : "Adding..."}
    >
      <TextControl<FormValues>
        name="name"
        label="Name"
        placeholder="scscf"
        disabled={editing}
        autoFocus={!editing}
      />
      <TextControl<FormValues>
        name="originHost"
        label={ORIGIN_HOST_LABEL}
        placeholder="scscf.ims.mnc001.mcc001.3gppnetwork.org"
        helperText={ORIGIN_HOST_HELP}
        autoFocus={editing}
      />
      <TextControl<FormValues>
        name="realm"
        label="Realm"
        placeholder="ims.mnc001.mcc001.3gppnetwork.org"
        helperText="The Diameter realm the peer identifies itself with. Optional."
      />
      <TextControl<FormValues>
        name="addresses"
        label="Source Addresses"
        placeholder="10.0.0.0/24, 2001:db8::/64"
        helperText="For example 10.6.0.5 or 10.6.0.0/24; use 0.0.0.0/0 to accept any IPv4 address. Separate with commas."
      />
    </FormDialog>
  );
};

export default IMSPeerModal;
