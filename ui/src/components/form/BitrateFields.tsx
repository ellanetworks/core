// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Box } from "@mui/material";
import type { FieldValues, Path } from "react-hook-form";
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
