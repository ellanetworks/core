// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Checkbox, FormControlLabel, FormGroup } from "@mui/material";
import { useController } from "react-hook-form";
import type { Control, FieldValues, Path } from "react-hook-form";
import type { AmbrUnit } from "@/components/form/BitrateFields";

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
