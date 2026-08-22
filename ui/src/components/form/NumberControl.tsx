// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { TextField } from "@mui/material";
import type { TextFieldProps } from "@mui/material";
import {
  useController,
  type FieldPath,
  type FieldValues,
} from "react-hook-form";

type NumberControlProps<
  T extends FieldValues,
  P extends FieldPath<T> = FieldPath<T>,
> = Omit<
  TextFieldProps,
  "name" | "value" | "onChange" | "onBlur" | "error" | "type"
> & {
  name: P;
  label: string;
  min?: number;
  max?: number;
  showErrorWhileTyping?: boolean;
};

const NumberControl = <
  T extends FieldValues,
  P extends FieldPath<T> = FieldPath<T>,
>({
  name,
  label,
  min,
  max,
  helperText,
  showErrorWhileTyping = false,
  ...rest
}: NumberControlProps<T, P>) => {
  const { field, fieldState } = useController<T, P>({ name });
  const showError =
    !!fieldState.error && (fieldState.isTouched || showErrorWhileTyping);

  return (
    <TextField
      {...rest}
      name={field.name}
      inputRef={field.ref}
      onBlur={field.onBlur}
      value={field.value ?? ""}
      onChange={(event) =>
        field.onChange(
          event.target.value === "" ? undefined : Number(event.target.value),
        )
      }
      type="number"
      label={label}
      fullWidth
      margin="normal"
      error={showError}
      helperText={showError ? fieldState.error?.message : helperText}
      slotProps={{ input: { inputProps: { min, max } } }}
    />
  );
};

export default NumberControl;
