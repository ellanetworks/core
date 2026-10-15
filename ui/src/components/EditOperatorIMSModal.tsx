// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  MAX_PCSCF_ADDRESSES_PER_FAMILY,
  updateOperatorIMS,
} from "@/queries/operator";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import { ipv4Regex, ipv6Regex } from "@/utils/ip";

interface EditOperatorIMSModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: { pcscfAddresses: string[] };
}

export const splitAddresses = (value: string | undefined) =>
  (value ?? "").split(/[\s,]+/).filter(Boolean);

export const schema = yup.object({
  pcscfAddresses: yup
    .string()
    .default("")
    .test("format", function (value) {
      const addresses = splitAddresses(value);
      const invalid = addresses.find(
        (a) => !ipv4Regex.test(a) && !ipv6Regex.test(a),
      );
      if (invalid) {
        return this.createError({
          message: `${invalid} is not an IPv4 or IPv6 address`,
        });
      }

      const duplicate = addresses.find((a, i) => addresses.indexOf(a) !== i);
      if (duplicate) {
        return this.createError({
          message: `${duplicate} is listed more than once`,
        });
      }

      const ipv4 = addresses.filter((a) => ipv4Regex.test(a)).length;
      if (
        ipv4 > MAX_PCSCF_ADDRESSES_PER_FAMILY ||
        addresses.length - ipv4 > MAX_PCSCF_ADDRESSES_PER_FAMILY
      ) {
        return this.createError({
          message: `At most ${MAX_PCSCF_ADDRESSES_PER_FAMILY} IPv4 and ${MAX_PCSCF_ADDRESSES_PER_FAMILY} IPv6 addresses`,
        });
      }

      return true;
    }),
});

type FormValues = yup.InferType<typeof schema>;

const EditOperatorIMSModal: React.FC<EditOperatorIMSModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();
  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: { pcscfAddresses: initialData.pcscfAddresses.join(", ") },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateOperatorIMS(accessToken, {
      pcscfAddresses: splitAddresses(values.pcscfAddresses),
    });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="P-CSCF Addresses"
      description="The IMS entry points handsets send their calls to, in order of preference. Handsets on the ims data network receive them when they connect."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update voice settings"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <TextControl<FormValues>
        name="pcscfAddresses"
        label="Addresses"
        placeholder="10.6.0.5, 2001:db8::5"
        helperText="Separate addresses with commas. Leave empty to turn voice off."
        autoFocus
      />
    </FormDialog>
  );
};

export default EditOperatorIMSModal;
