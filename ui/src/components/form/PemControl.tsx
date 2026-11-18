// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useRef, type ChangeEvent } from "react";
import { Box, IconButton, InputAdornment, Tooltip } from "@mui/material";
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
  minRows?: number;
  maxRows?: number;
}

const PemControl = <T extends FieldValues>({
  name,
  label,
  helperText,
  placeholder = "-----BEGIN CERTIFICATE-----",
  accept = ".pem,.crt,.cer,.key",
  minRows = 3,
  maxRows = 8,
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
        minRows={minRows}
        maxRows={maxRows}
        slotProps={{
          htmlInput: {
            spellCheck: false,
            style: { fontFamily: "monospace", fontSize: 12 },
          },
          input: {
            endAdornment: (
              <InputAdornment position="end" sx={{ alignSelf: "flex-start" }}>
                <Tooltip title="Upload file">
                  <IconButton
                    size="small"
                    edge="end"
                    aria-label={`Upload ${label}`}
                    onClick={() => inputRef.current?.click()}
                  >
                    <UploadFileIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
              </InputAdornment>
            ),
          },
        }}
      />
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
