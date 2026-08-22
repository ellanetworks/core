// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Box, Button, TextField } from "@mui/material";
import { useController, useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { createHomeNetworkKey } from "@/queries/operator";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import NumberControl from "@/components/form/NumberControl";
import SelectControl from "@/components/form/SelectControl";

interface CreateHomeNetworkKeyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object({
  keyIdentifier: yup
    .number()
    .typeError("Key Identifier must be a number.")
    .min(0, "Key Identifier must be between 0 and 255.")
    .max(255, "Key Identifier must be between 0 and 255.")
    .required("Key Identifier is required."),
  scheme: yup
    .string()
    .oneOf(["A", "B"], 'Scheme must be "A" or "B".')
    .required("Scheme is required."),
  privateKey: yup
    .string()
    .matches(
      /^[a-fA-F0-9]{64}$/,
      "Private Key must be a 64-character hexadecimal string.",
    )
    .required("Private Key is required."),
});

type FormValues = yup.InferType<typeof schema>;

const SCHEME_OPTIONS = [
  { value: "A", label: "Profile A (X25519)" },
  { value: "B", label: "Profile B (P-256)" },
] as const;

const randomPrivateKey = () => {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return Array.from(bytes)
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
};

const CreateHomeNetworkKeyModal: React.FC<CreateHomeNetworkKeyModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: { keyIdentifier: 0, scheme: "A", privateKey: "" },
  });

  const { field: privateKeyField, fieldState: privateKeyState } = useController(
    { control: form.control, name: "privateKey" },
  );
  const showKeyError = !!privateKeyState.error && privateKeyState.isTouched;

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await createHomeNetworkKey(
      accessToken,
      Number(values.keyIdentifier),
      values.scheme,
      values.privateKey,
    );
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Add Home Network Key"
      description="Configure a home network key for SUCI de-concealment. The key identifier and scheme must match the values provisioned on the subscriber's SIM card."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to create home network key"
      submitLabel="Create"
      submittingLabel="Creating..."
      fullWidth={false}
    >
      <NumberControl<FormValues>
        name="keyIdentifier"
        label="Key Identifier"
        min={0}
        max={255}
        helperText="0-255. Must match the value provisioned on the SIM/USIM."
        autoFocus
      />
      <SelectControl<FormValues, string>
        name="scheme"
        label="Scheme"
        options={SCHEME_OPTIONS}
      />
      <Box sx={{ display: "flex", gap: 2, alignItems: "flex-start" }}>
        <TextField
          fullWidth
          label="Private Key"
          value={privateKeyField.value ?? ""}
          onChange={privateKeyField.onChange}
          onBlur={privateKeyField.onBlur}
          inputRef={privateKeyField.ref}
          error={showKeyError}
          helperText={showKeyError ? privateKeyState.error?.message : " "}
          margin="normal"
          sx={{
            flex: 1,
            "& .MuiInputBase-input": {
              textOverflow: "ellipsis",
              overflow: "hidden",
              whiteSpace: "nowrap",
            },
          }}
        />
        <Button
          variant="contained"
          color="primary"
          sx={{
            flex: "0 0 120px",
            minWidth: 120,
            flexShrink: 0,
            mt: "16px",
            height: "56px",
          }}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() =>
            form.setValue("privateKey", randomPrivateKey(), {
              shouldValidate: true,
              shouldTouch: true,
            })
          }
        >
          Generate
        </Button>
      </Box>
    </FormDialog>
  );
};

export default CreateHomeNetworkKeyModal;
