// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { TextField } from "@mui/material";
import type { TextFieldProps } from "@mui/material";
import {
  useController,
  type FieldPath,
  type FieldValues,
} from "react-hook-form";

type TextControlProps<
  T extends FieldValues,
  P extends FieldPath<T> = FieldPath<T>,
> = Omit<TextFieldProps, "name" | "value" | "onChange" | "onBlur" | "error"> & {
  name: P;
  label: string;
  showErrorWhileTyping?: boolean;
};

const TextControl = <
  T extends FieldValues,
  P extends FieldPath<T> = FieldPath<T>,
>({
  name,
  label,
  showErrorWhileTyping = false,
  helperText,
  ...rest
}: TextControlProps<T, P>) => {
  const { field, fieldState } = useController<T, P>({ name });
  const showError =
    !!fieldState.error && (fieldState.isTouched || showErrorWhileTyping);

  return (
    <TextField
      {...rest}
      {...field}
      value={field.value ?? ""}
      label={label}
      fullWidth
      margin="normal"
      error={showError}
      helperText={showError ? fieldState.error?.message : helperText}
    />
  );
};

export default TextControl;
