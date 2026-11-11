// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateSMSSettings } from "@/queries/integrations";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import { msisdnSchema } from "@/components/subscriberIdentity";
import { PRODUCT } from "@/utils/product";

export const SERVING_NODE_NUMBER_HELP = `E.164 number ${PRODUCT.name} gives the SMSC as the serving MME/SMSF address for mobile-terminated SMS.`;

interface EditServingNodeNumberModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: { servingNodeNumber: string };
}

const schema = yup.object({
  servingNodeNumber: msisdnSchema.test(
    "required",
    "Serving node number is required",
    (v) => !!v?.trim(),
  ),
});

type FormValues = yup.InferType<typeof schema>;

const EditServingNodeNumberModal: React.FC<EditServingNodeNumberModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();
  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: { servingNodeNumber: initialData.servingNodeNumber },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateSMSSettings(accessToken, {
      servingNodeNumber: values.servingNodeNumber.trim(),
    });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Serving Node Number"
      description={SERVING_NODE_NUMBER_HELP}
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update SMS settings"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <TextControl<FormValues>
        name="servingNodeNumber"
        label="E.164 Number"
        placeholder="+15550001111"
        helperText="For example +15550001111."
        autoFocus
      />
    </FormDialog>
  );
};

export default EditServingNodeNumberModal;
