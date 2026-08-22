// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect } from "react";
import { Checkbox, FormControlLabel } from "@mui/material";
import { useController, useForm, useWatch } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { createRoute } from "@/queries/routes";
import { useAuth } from "@/contexts/AuthContext";
import { ipRegex, isValidCidr } from "@/utils/ip";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import NumberControl from "@/components/form/NumberControl";
import SelectControl from "@/components/form/SelectControl";

interface CreateRouteModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object({
  defaultRoute: yup.boolean().required(),
  destination: yup
    .string()
    .default("")
    .when(["defaultRoute"], (values, destinationSchema) => {
      const defaultRoute = values[0] as boolean;
      if (defaultRoute) {
        return destinationSchema.test(
          "default-route",
          "For a default route, destination must be 0.0.0.0/0 or ::/0",
          (value) => value === "0.0.0.0/0" || value === "::/0",
        );
      }
      return destinationSchema
        .required("Destination is required")
        .test(
          "valid-cidr",
          "Destination must be a valid CIDR (IPv4 or IPv6)",
          (value) => value != null && value !== "" && isValidCidr(value),
        );
    }),
  gateway: yup
    .string()
    .required("Gateway is required")
    .test(
      "valid-gateway",
      "Gateway must be a valid IPv4 or IPv6 address",
      (value) => value != null && ipRegex.test(value),
    ),
  interface: yup
    .string()
    .oneOf(["n3", "n6"], "Interface must be either n3 or n6")
    .required("Interface is required"),
  metric: yup.number().required("Metric is required"),
});

type FormValues = yup.InferType<typeof schema>;

const INTERFACE_OPTIONS = [
  { value: "n3", label: "n3" },
  { value: "n6", label: "n6" },
] as const;

const defaultDestinationFor = (gateway: string) =>
  gateway.includes(":") ? "::/0" : "0.0.0.0/0";

const CreateRouteModal: React.FC<CreateRouteModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: {
      destination: "",
      gateway: "",
      interface: "n6",
      metric: 0,
      defaultRoute: false,
    },
  });

  const { field: defaultRouteField } = useController({
    control: form.control,
    name: "defaultRoute",
  });
  const defaultRoute = useWatch({
    control: form.control,
    name: "defaultRoute",
  });
  const gateway = useWatch({ control: form.control, name: "gateway" });

  useEffect(() => {
    if (defaultRoute) {
      form.setValue("destination", defaultDestinationFor(gateway ?? ""), {
        shouldValidate: true,
      });
    }
  }, [defaultRoute, gateway, form]);

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await createRoute(
      accessToken,
      values.destination,
      values.gateway,
      values.interface,
      values.metric,
    );
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Create Route"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to create route"
      submitLabel="Create"
      submittingLabel="Creating..."
      fullWidth={false}
    >
      <FormControlLabel
        control={
          <Checkbox
            checked={defaultRouteField.value}
            onChange={(event) => {
              const checked = event.target.checked;
              defaultRouteField.onChange(checked);
              if (!checked) {
                form.setValue("destination", "", { shouldValidate: true });
              }
            }}
          />
        }
        label="Default Route (0.0.0.0/0 or ::/0)"
      />
      <TextControl<FormValues>
        name="destination"
        label="Destination"
        disabled={defaultRoute}
        autoFocus
      />
      <TextControl<FormValues> name="gateway" label="Gateway" />
      <SelectControl<FormValues, string>
        name="interface"
        label="Interface"
        options={INTERFACE_OPTIONS}
      />
      <NumberControl<FormValues> name="metric" label="Metric" />
    </FormDialog>
  );
};

export default CreateRouteModal;
