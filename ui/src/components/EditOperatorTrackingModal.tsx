// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { Autocomplete, Chip, TextField } from "@mui/material";
import { useController, useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { updateOperatorTracking } from "@/queries/operator";
import { useAuth } from "@/contexts/AuthContext";
import { TAC_HEX_PATTERN, formatTacDecimal } from "@/utils/tac";
import TacValue from "@/components/TacValue";
import FormDialog from "@/components/form/FormDialog";

interface EditOperatorTrackingModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    supportedTacs: string[];
  };
}

const tacSchema = yup
  .string()
  .matches(
    TAC_HEX_PATTERN,
    "Each TAC must be a 3 bytes hex string, range: 000000~FFFFFF)",
  );

const schema = yup.object({
  supportedTacs: yup
    .array()
    .of(yup.string().required())
    .required()
    .test("valid-tacs", function (tacs) {
      const invalid = (tacs ?? []).filter((tac) => !tacSchema.isValidSync(tac));
      return invalid.length === 0
        ? true
        : this.createError({
            message: `Invalid TACs: ${invalid.join(", ")}`,
          });
    }),
});

type FormValues = yup.InferType<typeof schema>;

const DEFAULT_TAC_HELPER_TEXT =
  "Enter each TAC as a 3 bytes hex string (e.g., 000001)";

const decimalHint = (input: string): string | null => {
  const trimmed = input.trim();

  if (!TAC_HEX_PATTERN.test(trimmed)) return null;

  const decimal = formatTacDecimal(trimmed);

  return decimal === null ? null : `${trimmed} is ${decimal} in decimal`;
};

const EditOperatorTrackingModal: React.FC<EditOperatorTrackingModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const { accessToken } = useAuth();
  const [inputValue, setInputValue] = useState("");

  const form = useForm<FormValues>({
    mode: "onChange",
    resolver: yupResolver(schema),
    values: { supportedTacs: initialData.supportedTacs },
  });

  const { field, fieldState } = useController({
    control: form.control,
    name: "supportedTacs",
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    const pending = inputValue.trim();
    const tacs = pending
      ? [...values.supportedTacs, pending]
      : values.supportedTacs;

    const invalid = tacs.filter((tac) => !tacSchema.isValidSync(tac));
    if (invalid.length > 0) {
      form.setError("supportedTacs", {
        message: `Invalid TACs: ${invalid.join(", ")}`,
      });
      return false;
    }

    await updateOperatorTracking(accessToken, tacs);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Operator Tracking Information"
      description="Tracking Area Codes (TACs) are used to identify a tracking area in a mobile network. Only radios with TACs listed here will be able to connect to the network."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update supported TACs"
      submitLabel="Update"
      submittingLabel="Updating..."
      fullWidth={false}
    >
      <Autocomplete
        multiple
        freeSolo
        options={[]}
        value={field.value}
        onChange={(_event, value) => field.onChange(value)}
        onBlur={field.onBlur}
        inputValue={inputValue}
        onInputChange={(_event, value) => setInputValue(value)}
        renderValue={(tacs, getItemProps) =>
          tacs.map((tac, index) => {
            const { key, ...itemProps } = getItemProps({ index });

            return (
              <Chip key={key} label={<TacValue tac={tac} />} {...itemProps} />
            );
          })
        }
        renderInput={(params) => (
          <TextField
            {...params}
            variant="outlined"
            label="Supported TACs"
            placeholder="Enter TACs (e.g., 000001)"
            error={!!fieldState.error}
            helperText={
              fieldState.error?.message ||
              decimalHint(inputValue) ||
              DEFAULT_TAC_HELPER_TEXT
            }
            autoFocus
          />
        )}
        sx={{ marginBottom: 2 }}
      />
    </FormDialog>
  );
};

export default EditOperatorTrackingModal;
