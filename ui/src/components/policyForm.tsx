// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect } from "react";
import { Typography } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { useController, useWatch } from "react-hook-form";
import type { Control, FieldValues, Path } from "react-hook-form";
import * as yup from "yup";
import { useAuth } from "@/contexts/AuthContext";
import { listDataNetworks } from "@/queries/data_networks";
import { listSlices } from "@/queries/slices";
import SelectControl from "@/components/form/SelectControl";
import NumberControl from "@/components/form/NumberControl";
import { AmbrFields, ambrSchema } from "@/components/form/BitrateFields";
import type { AmbrUnit } from "@/components/form/BitrateFields";
import { IMS_DATA_NETWORK_5QIS, IMS_SIGNALLING_5QI } from "@/utils/voice";

const PER_PAGE = 100;

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

const NO_PRIORITY_ARP = 15;

const NO_PRIORITY_MESSAGE =
  "ARP 15 means no priority on 4G, where pre-emption is ignored. Use an ARP between 1 and 14.";

export type PreemptionCapability = "may_preempt" | "not_preempt";

export type PreemptionVulnerability = "preemptable" | "not_preemptable";

const PREEMPTION_CAPABILITY_OPTIONS: {
  value: PreemptionCapability;
  label: string;
}[] = [
  { value: "not_preempt", label: "Shall not pre-empt" },
  { value: "may_preempt", label: "May pre-empt" },
];

const PREEMPTION_VULNERABILITY_OPTIONS: {
  value: PreemptionVulnerability;
  label: string;
}[] = [
  { value: "not_preemptable", label: "Not pre-emptable" },
  { value: "preemptable", label: "Pre-emptable" },
];

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
  preemptionCapability: yup
    .mixed<PreemptionCapability>()
    .oneOf(["may_preempt", "not_preempt"])
    .test(
      "no-priority",
      NO_PRIORITY_MESSAGE,
      (value, ctx) =>
        ctx.parent.arp !== NO_PRIORITY_ARP || value === "not_preempt",
    )
    .required(),
  preemptionVulnerability: yup
    .mixed<PreemptionVulnerability>()
    .oneOf(["preemptable", "not_preemptable"])
    .test(
      "no-priority",
      NO_PRIORITY_MESSAGE,
      (value, ctx) =>
        ctx.parent.arp !== NO_PRIORITY_ARP || value === "not_preemptable",
    )
    .required(),
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

const listAll = async <I extends { name: string }>(
  fetchPage: (page: number) => Promise<{ items?: I[]; total_count?: number }>,
) => {
  const all: I[] = [];

  for (let page = 1; ; page++) {
    const result = await fetchPage(page);
    const items = result.items ?? [];
    all.push(...items);

    if (items.length < PER_PAGE || all.length >= (result.total_count ?? 0)) {
      return all;
    }
  }
};

export const useNetworkOptions = (open: boolean) => {
  const { accessToken, authReady } = useAuth();
  const enabled = open && authReady && !!accessToken;

  const dataNetworksQuery = useQuery({
    queryKey: ["policy-data-networks"],
    queryFn: () =>
      listAll((page) => listDataNetworks(accessToken!, page, PER_PAGE)),
    enabled,
  });

  const slicesQuery = useQuery({
    queryKey: ["policy-slices"],
    queryFn: () => listAll((page) => listSlices(accessToken!, page, PER_PAGE)),
    enabled,
  });

  return {
    dataNetworks: dataNetworksQuery.data?.map((dn) => dn.name),
    imsDataNetworks: dataNetworksQuery.data
      ?.filter((dn) => dn.ims)
      .map((dn) => dn.name),
    slices: slicesQuery.data?.map((slice) => slice.name),
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
  imsDataNetworks,
}: {
  control: Control<T>;
  name: Path<T>;
  imsDataNetworks: string[];
}) => {
  const { field } = useController({ control, name });
  const dataNetwork = useWatch({
    control,
    name: "dataNetworkName" as Path<T>,
  }) as string | undefined;
  const ims = !!dataNetwork && imsDataNetworks.includes(dataNetwork);
  const current = field.value as number | undefined;
  const { onChange } = field;

  useEffect(() => {
    if (
      ims &&
      (current === undefined || !IMS_DATA_NETWORK_5QIS.includes(current))
    )
      onChange(IMS_SIGNALLING_5QI);
  }, [ims, current, onChange]);

  const allowed = ims
    ? NON_GBR_5QI_OPTIONS.filter((o) => IMS_DATA_NETWORK_5QIS.includes(o.value))
    : NON_GBR_5QI_OPTIONS;
  const options =
    current !== undefined && !allowed.some((o) => o.value === current)
      ? [{ value: current, label: String(current) }, ...allowed]
      : allowed;

  return (
    <>
      <SelectControl<T, number>
        name={name}
        label="5QI / QCI"
        options={options}
        numeric
      />
      <Typography variant="caption" color="textSecondary">
        {ims
          ? "IMS signalling uses 5QI 5, or 69 for mission critical services."
          : "Determines radio scheduling behavior. Only non-GBR classes are supported."}
      </Typography>
    </>
  );
};

interface PolicyFieldsProps<T extends FieldValues> {
  control: Control<T>;
  dataNetworks: string[];
  imsDataNetworks: string[];
  slices: string[];
}

export const PolicyFields = <T extends FieldValues>({
  control,
  dataNetworks,
  imsDataNetworks,
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
    <FiveQiSelect
      control={control}
      name={"fiveQi" as Path<T>}
      imsDataNetworks={imsDataNetworks}
    />
    <NumberControl<T>
      name={"arp" as Path<T>}
      label="Allocation and Retention Priority (ARP)"
      helperText="Admission control priority at session setup. 1 (highest) to 15 (lowest). Levels 1 to 8 are meant for prioritized services."
    />
    <SelectControl<T, PreemptionCapability>
      name={"preemptionCapability" as Path<T>}
      label="Pre-emption capability"
      options={PREEMPTION_CAPABILITY_OPTIONS}
    />
    <SelectControl<T, PreemptionVulnerability>
      name={"preemptionVulnerability" as Path<T>}
      label="Pre-emption vulnerability"
      options={PREEMPTION_VULNERABILITY_OPTIONS}
    />
    <Typography variant="caption" color="textSecondary">
      A pre-emptable session can be dropped by the radio when a higher priority
      session needs its resources.
    </Typography>
  </>
);
