// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Typography } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { useController } from "react-hook-form";
import type { Control, FieldValues, Path } from "react-hook-form";
import * as yup from "yup";
import { useAuth } from "@/contexts/AuthContext";
import { listDataNetworks } from "@/queries/data_networks";
import { listSlices } from "@/queries/slices";
import SelectControl from "@/components/form/SelectControl";
import NumberControl from "@/components/form/NumberControl";
import { AmbrFields, ambrSchema } from "@/components/form/BitrateFields";
import type { AmbrUnit } from "@/components/form/BitrateFields";

const PER_PAGE = 12;

export const NON_GBR_5QI_OPTIONS: { value: number; label: string }[] = [
  { value: 5, label: "5 — IMS signalling" },
  { value: 6, label: "6 — Buffered streaming, web browsing" },
  { value: 7, label: "7 — Voice, live video, interactive gaming" },
  { value: 8, label: "8 — Buffered streaming" },
  { value: 9, label: "9 — Best effort (default)" },
  { value: 69, label: "69 — Mission critical signalling" },
  { value: 70, label: "70 — Mission critical data" },
  { value: 79, label: "79 — V2X messages" },
  { value: 80, label: "80 — Low latency eMBB" },
];

export const NON_GBR_5QI_VALUES = NON_GBR_5QI_OPTIONS.map((o) => o.value);

export const policySchema = {
  sliceName: yup.string().required("Slice is required."),
  dataNetworkName: yup.string().required("Data Network is required."),
  ...ambrSchema,
  fiveQi: yup
    .number()
    .oneOf(
      NON_GBR_5QI_VALUES,
      `5QI must be one of: ${NON_GBR_5QI_VALUES.join(", ")}`,
    )
    .required("5QI is required"),
  arp: yup.number().min(1).max(15).required("ARP is required"),
  isDefault: yup.boolean().required(),
};

export const splitAmbr = (value: string): { num: number; unit: AmbrUnit } => {
  const [rawValue, rawUnit] = value.split(" ");
  const num = parseInt(rawValue, 10);
  const unit =
    rawUnit === "Kbps" || rawUnit === "Mbps" || rawUnit === "Gbps"
      ? rawUnit
      : "Mbps";
  return { num: Number.isNaN(num) ? 100 : num, unit };
};

export const useNetworkOptions = (open: boolean) => {
  const { accessToken, authReady } = useAuth();
  const enabled = open && authReady && !!accessToken;

  const dataNetworksQuery = useQuery({
    queryKey: ["policy-data-networks"],
    queryFn: async () => {
      const page = await listDataNetworks(accessToken!, 1, PER_PAGE);
      return (page.items ?? []).map((dn) => dn.name);
    },
    enabled,
  });

  const slicesQuery = useQuery({
    queryKey: ["policy-slices"],
    queryFn: async () => {
      const page = await listSlices(accessToken!, 1, PER_PAGE);
      return (page.items ?? []).map((slice) => slice.name);
    },
    enabled,
  });

  return {
    dataNetworks: dataNetworksQuery.data,
    slices: slicesQuery.data,
    isError: dataNetworksQuery.isError || slicesQuery.isError,
  };
};

const withCurrent = (options: string[], current: string | undefined) =>
  current && !options.includes(current) ? [current, ...options] : options;

interface NameSelectProps<T extends FieldValues> {
  control: Control<T>;
  name: Path<T>;
  label: string;
  options: string[];
}

const NameSelect = <T extends FieldValues>({
  control,
  name,
  label,
  options,
}: NameSelectProps<T>) => {
  const { field } = useController({ control, name });
  const choices = withCurrent(options, field.value as string | undefined);

  return (
    <SelectControl<T, string>
      name={name}
      label={label}
      options={choices.map((choice) => ({ value: choice, label: choice }))}
    />
  );
};

const FiveQiSelect = <T extends FieldValues>({
  control,
  name,
}: {
  control: Control<T>;
  name: Path<T>;
}) => {
  const { field } = useController({ control, name });
  const current = field.value as number | undefined;
  const options =
    current !== undefined && !NON_GBR_5QI_VALUES.includes(current)
      ? [{ value: current, label: String(current) }, ...NON_GBR_5QI_OPTIONS]
      : NON_GBR_5QI_OPTIONS;

  return (
    <>
      <SelectControl<T, number>
        name={name}
        label="5QI / QCI"
        options={options}
        numeric
      />
      <Typography variant="caption" color="textSecondary">
        Determines radio scheduling behavior. Only non-GBR classes are
        supported.
      </Typography>
    </>
  );
};

interface PolicyFieldsProps<T extends FieldValues> {
  control: Control<T>;
  dataNetworks: string[];
  slices: string[];
}

export const PolicyFields = <T extends FieldValues>({
  control,
  dataNetworks,
  slices,
}: PolicyFieldsProps<T>) => (
  <>
    <NameSelect
      control={control}
      name={"sliceName" as Path<T>}
      label="Slice"
      options={slices}
    />
    <NameSelect
      control={control}
      name={"dataNetworkName" as Path<T>}
      label="Data Network"
      options={dataNetworks}
    />
    <AmbrFields<T>
      valueName={"ambrUpValue" as Path<T>}
      unitName={"ambrUpUnit" as Path<T>}
      label="Session Bitrate Uplink"
    />
    <AmbrFields<T>
      valueName={"ambrDownValue" as Path<T>}
      unitName={"ambrDownUnit" as Path<T>}
      label="Session Bitrate Downlink"
    />
    <FiveQiSelect control={control} name={"fiveQi" as Path<T>} />
    <NumberControl<T>
      name={"arp" as Path<T>}
      label="Allocation and Retention Priority (ARP)"
      helperText="Admission control priority at session setup. 1 (highest) to 15 (lowest)."
    />
  </>
);
