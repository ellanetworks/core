// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useRef, type ChangeEvent } from "react";
import { Box, Button } from "@mui/material";
import { UploadFile as UploadFileIcon } from "@mui/icons-material";
import {
  useFormContext,
  type FieldPath,
  type FieldValues,
  type PathValue,
} from "react-hook-form";
import TextControl from "@/components/form/TextControl";

interface PemControlProps<T extends FieldValues> {
  name: FieldPath<T>;
  label: string;
  helperText?: string;
  placeholder?: string;
  accept?: string;
}

const PemControl = <T extends FieldValues>({
  name,
  label,
  helperText,
  placeholder = "-----BEGIN CERTIFICATE-----",
  accept = ".pem,.crt,.cer,.key",
}: PemControlProps<T>) => {
  const { setValue } = useFormContext<T>();
  const inputRef = useRef<HTMLInputElement>(null);

  const handleFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    const text = await file.text();
    setValue(name, text as PathValue<T, FieldPath<T>>, {
      shouldValidate: true,
      shouldTouch: true,
      shouldDirty: true,
    });
  };

  return (
    <Box>
      <TextControl<T>
        name={name}
        label={label}
        helperText={helperText}
        placeholder={placeholder}
        multiline
        minRows={3}
        maxRows={8}
        slotProps={{
          htmlInput: {
            spellCheck: false,
            style: { fontFamily: "monospace", fontSize: 12 },
          },
        }}
      />
      <Button
        size="small"
        startIcon={<UploadFileIcon />}
        onClick={() => inputRef.current?.click()}
      >
        Upload {label}
      </Button>
      <input
        ref={inputRef}
        type="file"
        accept={accept}
        hidden
        aria-label={`${label} file`}
        onChange={handleFile}
      />
    </Box>
  );
};

export default PemControl;
