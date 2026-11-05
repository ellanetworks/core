// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { TextField } from "@mui/material";
import { useController } from "react-hook-form";
import type { Control, FieldValues, Path } from "react-hook-form";
import SubscriberSearchField from "@/components/SubscriberSearchField";

interface SubscriberSelectFieldProps<T extends FieldValues> {
  control: Control<T>;
  name: Path<T>;
  dataNetwork: string;
  open: boolean;
  readOnly?: boolean;
}

const SubscriberSelectField = <T extends FieldValues>({
  control,
  name,
  dataNetwork,
  open,
  readOnly = false,
}: SubscriberSelectFieldProps<T>) => {
  const { field } = useController({ control, name });

  if (readOnly) {
    return (
      <TextField
        fullWidth
        label="Subscriber"
        value={field.value ?? ""}
        margin="normal"
        disabled
      />
    );
  }

  return (
    <SubscriberSearchField
      value={field.value ?? ""}
      onChange={field.onChange}
      onBlur={field.onBlur}
      dataNetwork={dataNetwork}
      enabled={open}
      margin="normal"
      autoFocus
      showError
    />
  );
};

export default SubscriberSelectField;
