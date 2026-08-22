// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useId } from "react";
import { FormControl, InputLabel, MenuItem, Select } from "@mui/material";
import {
  useController,
  type FieldPath,
  type FieldValues,
} from "react-hook-form";

interface SelectOption<V extends string | number> {
  value: V;
  label: string;
}

interface SelectControlProps<
  T extends FieldValues,
  V extends string | number,
  P extends FieldPath<T> = FieldPath<T>,
> {
  name: P;
  label: string;
  options: readonly SelectOption<V>[];
  numeric?: boolean;
  autoFocus?: boolean;
  disabled?: boolean;
}

const SelectControl = <
  T extends FieldValues,
  V extends string | number,
  P extends FieldPath<T> = FieldPath<T>,
>({
  name,
  label,
  options,
  numeric = false,
  autoFocus,
  disabled,
}: SelectControlProps<T, V, P>) => {
  const labelId = useId();
  const { field } = useController<T, P>({ name });

  return (
    <FormControl fullWidth margin="normal" disabled={disabled}>
      <InputLabel id={labelId}>{label}</InputLabel>
      <Select
        labelId={labelId}
        label={label}
        value={field.value ?? ""}
        onChange={(event) =>
          field.onChange(
            numeric ? Number(event.target.value) : event.target.value,
          )
        }
        onBlur={field.onBlur}
        inputRef={field.ref}
        autoFocus={autoFocus}
      >
        {options.map((option) => (
          <MenuItem key={option.value} value={option.value}>
            {option.label}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
};

export default SelectControl;
