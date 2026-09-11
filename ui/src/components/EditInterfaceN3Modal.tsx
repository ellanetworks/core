// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateN3Settings } from "@/queries/interfaces";
import { useAuth } from "@/contexts/AuthContext";
import { ipv4Regex, ipv6Regex } from "@/utils/ip";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import { PRODUCT } from "@/utils/product";

interface EditInterfaceN3ModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    externalAddress: string;
  };
}

const schema = yup.object({
  externalAddress: yup
    .string()
    .trim()
    .default("")
    .test(
      "empty-or-ipv4-or-ipv6",
      "External address must be an IPv4 address, an IPv6 address, or one of each separated by a comma",
      (value) => {
        if (!value) return true;

        const parts = value.split(",").map((part) => part.trim());
        if (parts.length > 2) return false;

        const v4 = parts.filter((part) => ipv4Regex.test(part));
        const v6 = parts.filter((part) => ipv6Regex.test(part));

        return (
          v4.length + v6.length === parts.length &&
          v4.length <= 1 &&
          v6.length <= 1
        );
      },
    ),
});

type FormValues = yup.InferType<typeof schema>;

const EditInterfaceN3Modal: React.FC<EditInterfaceN3ModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onChange",
    resolver: yupResolver(schema),
    values: { externalAddress: initialData.externalAddress },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateN3Settings(accessToken, values.externalAddress || "");
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit N3 Interface"
      description={`Configure an external address (IPv4 or IPv6) for N3. ${PRODUCT.name} will advertise this address to radios which will use it to establish GTP tunnels. Use this if ${PRODUCT.name} is behind a proxy, NAT, or load-balancer. If not set, ${PRODUCT.name} will use N3's address as defined in the config file.`}
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update N3 external address"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <TextControl<FormValues>
        name="externalAddress"
        label="External Address"
        helperText="Leave empty to use N3's configured address. Supports both IPv4 and IPv6."
        showErrorWhileTyping
        autoFocus
      />
    </FormDialog>
  );
};

export default EditInterfaceN3Modal;
