// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo } from "react";
import { useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  DEFAULT_SMSC_PORT,
  updateOperatorSMS,
  type OperatorSMS,
} from "@/queries/operator";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import NumberControl from "@/components/form/NumberControl";
import { isHostAddress } from "@/utils/ip";
import { msisdnSchema } from "@/components/subscriberIdentity";
import { PRODUCT } from "@/utils/product";

interface EditOperatorSMSModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: OperatorSMS;
}

const makeSchema = (enabled: boolean) =>
  yup.object({
    smscAddress: yup
      .string()
      .default("")
      .test(
        "required",
        "SMSC address is required while SMS is on",
        (value) => !enabled || !!value?.trim(),
      )
      .test(
        "smsc-address",
        "SMSC address must be an IPv4 or IPv6 address",
        (value) => !value?.trim() || isHostAddress(value.trim()),
      ),
    smscPort: yup
      .number()
      .typeError("SMSC port must be a number")
      .integer("SMSC port must be a whole number")
      .min(1, "SMSC port must be between 1 and 65535")
      .max(65535, "SMSC port must be between 1 and 65535")
      .required("SMSC port is required"),
    smsNumber: msisdnSchema.test(
      "required",
      "SMS number is required while SMS is on",
      (v) => !enabled || !!v?.trim(),
    ),
  });

type FormValues = yup.InferType<ReturnType<typeof makeSchema>>;

const EditOperatorSMSModal: React.FC<EditOperatorSMSModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();
  const schema = useMemo(
    () => makeSchema(initialData.enabled),
    [initialData.enabled],
  );

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: {
      smscAddress: initialData.smscAddress,
      smscPort: initialData.smscPort,
      smsNumber: initialData.smsNumber,
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateOperatorSMS(accessToken, {
      enabled: initialData.enabled,
      smscAddress: values.smscAddress.trim(),
      smscPort: values.smscPort,
      smsNumber: values.smsNumber.trim(),
    });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit SMS"
      description={`While SMS is on, every ${PRODUCT.name} node connects to this SMSC over Diameter (SGd and S6c).`}
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update SMS settings"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <TextControl<FormValues>
        name="smscAddress"
        label="SMSC Address"
        placeholder="192.0.2.10"
        helperText="IP address of the SMSC's Diameter endpoint."
        autoFocus
      />
      <NumberControl<FormValues>
        name="smscPort"
        label="SMSC Port"
        min={1}
        max={65535}
        helperText={`SCTP port of the SMSC's Diameter endpoint (default ${DEFAULT_SMSC_PORT}).`}
      />
      <TextControl<FormValues>
        name="smsNumber"
        label="SMS Number"
        placeholder="+15550001111"
        helperText={`${PRODUCT.name}'s E.164 number for SMS, for example +15550001111.`}
      />
    </FormDialog>
  );
};

export default EditOperatorSMSModal;
