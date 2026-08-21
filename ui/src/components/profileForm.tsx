// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Box, Checkbox, FormControlLabel, FormGroup } from "@mui/material";
import { useController } from "react-hook-form";
import type { Control, FieldValues, Path } from "react-hook-form";
import * as yup from "yup";
import NumberControl from "@/components/form/NumberControl";
import SelectControl from "@/components/form/SelectControl";

export const AMBR_UNITS = ["Kbps", "Mbps", "Gbps"] as const;

export type AmbrUnit = (typeof AMBR_UNITS)[number];

const UNIT_OPTIONS = AMBR_UNITS.map((unit) => ({ value: unit, label: unit }));

const bitrateValue = () =>
  yup
    .number()
    .min(1, "Value must be between 1 and 65535")
    .max(65535, "Value must be between 1 and 65535")
    .integer("Value must be a whole number")
    .required("Value is required");

const bitrateUnit = () =>
  yup
    .string()
    .oneOf([...AMBR_UNITS], "Invalid unit")
    .required();

export const ambrSchema = {
  ambrUpValue: bitrateValue(),
  ambrUpUnit: bitrateUnit(),
  ambrDownValue: bitrateValue(),
  ambrDownUnit: bitrateUnit(),
};

export function parseAmbr(value: string): { num: number; unit: AmbrUnit } {
  const parts = value.split(" ");
  if (parts.length === 2) {
    const num = Number(parts[0]);
    const unit = parts[1];
    if (!isNaN(num) && (unit === "Mbps" || unit === "Gbps")) {
      return { num, unit };
    }
  }
  return { num: 100, unit: "Mbps" };
}

interface AmbrFieldsProps<T extends FieldValues> {
  valueName: Path<T>;
  unitName: Path<T>;
  label: string;
}

export const AmbrFields = <T extends FieldValues>({
  valueName,
  unitName,
  label,
}: AmbrFieldsProps<T>) => (
  <Box sx={{ display: "flex", gap: 2 }}>
    <NumberControl<T> name={valueName} label={label} sx={{ flex: 2 }} />
    <Box sx={{ flex: 1 }}>
      <SelectControl<T, string>
        name={unitName}
        label="Unit"
        options={UNIT_OPTIONS}
      />
    </Box>
  </Box>
);

interface AccessCheckboxProps<T extends FieldValues> {
  control: Control<T>;
  name: Path<T>;
  label: string;
}

const AccessCheckbox = <T extends FieldValues>({
  control,
  name,
  label,
}: AccessCheckboxProps<T>) => {
  const { field } = useController({ control, name });
  return (
    <FormControlLabel
      control={
        <Checkbox
          checked={!!field.value}
          onChange={(event) => field.onChange(event.target.checked)}
        />
      }
      label={label}
    />
  );
};

interface AccessCheckboxesProps<T extends FieldValues> {
  control: Control<T>;
  allow4gName: Path<T>;
  allow5gName: Path<T>;
}

export const AccessCheckboxes = <T extends FieldValues>({
  control,
  allow4gName,
  allow5gName,
}: AccessCheckboxesProps<T>) => (
  <FormGroup>
    <AccessCheckbox
      control={control}
      name={allow4gName}
      label="Allow 4G access"
    />
    <AccessCheckbox
      control={control}
      name={allow5gName}
      label="Allow 5G access"
    />
  </FormGroup>
);
