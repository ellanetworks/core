// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useMemo, useState } from "react";
import {
  Alert,
  Box,
  Button,
  Checkbox,
  FormControlLabel,
  InputAdornment,
  TextField,
} from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { useController, useForm, useWatch } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import {
  createSubscriber,
  MAX_DESCRIPTION_LENGTH,
} from "@/queries/subscribers";
import { getOperator } from "@/queries/operator";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import ProfileSelectField, {
  useProfileNames,
} from "@/components/ProfileSelectField";
import {
  descriptionSchema,
  getMSINBounds,
  parseIMSIorMSIN,
  randomKey,
  randomMSIN,
} from "@/components/subscriberIdentity";

interface CreateSubscriberModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object({
  msin: yup
    .string()
    .matches(/^\d+$/, "MSIN must be numeric.")
    .test("msin-len", function (value) {
      const mncLength = this.options?.context?.mncLength ?? 2;
      const { min, max } = getMSINBounds(mncLength);
      if (!value) return this.createError({ message: "MSIN is required." });
      return (
        (value.length >= min && value.length <= max) ||
        this.createError({
          message:
            min === max
              ? `MSIN must be exactly ${min} digits long.`
              : `MSIN must be between ${min} and ${max} digits long.`,
        })
      );
    })
    .required("MSIN is required."),
  key: yup
    .string()
    .matches(
      /^[0-9a-fA-F]{32}$/,
      "Key must be a 32-character hexadecimal string.",
    )
    .required("Key is required."),
  sequenceNumber: yup
    .string()
    .matches(
      /^[0-9a-fA-F]{12}$/,
      "Sequence Number must be a 6-byte (12-char) hex string.",
    )
    .required("Sequence Number is required."),
  profileName: yup.string().required("Profile is required."),
  description: descriptionSchema,
  opc: yup
    .string()
    .default("")
    .matches(
      /(^$)|(^[0-9a-fA-F]{32}$)/,
      "OPC must be empty or a 32-character hex string.",
    ),
});

type FormValues = yup.InferType<typeof schema>;

const GENERATE_BUTTON_SX = {
  flex: "0 0 120px",
  minWidth: 120,
  flexShrink: 0,
  mt: "16px",
  height: "56px",
};

const CreateSubscriberModal: React.FC<CreateSubscriberModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const { accessToken, authReady } = useAuth();
  const [customOPC, setCustomOPC] = useState(false);
  const [imsiMismatch, setImsiMismatch] = useState<string | null>(null);

  const operatorQuery = useQuery({
    queryKey: ["operator"],
    queryFn: () => getOperator(accessToken!),
    enabled: open && authReady && !!accessToken,
  });
  const profilesQuery = useProfileNames(open);

  const mcc = operatorQuery.data?.id.mcc ?? "";
  const mnc = operatorQuery.data?.id.mnc ?? "";
  const profiles = useMemo(
    () => profilesQuery.data ?? [],
    [profilesQuery.data],
  );

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    context: { mncLength: mnc.length },
    defaultValues: {
      msin: "",
      key: "",
      opc: "",
      sequenceNumber: "000000000022",
      profileName: "",
      description: "",
    },
  });

  const { field: msinField, fieldState: msinState } = useController({
    control: form.control,
    name: "msin",
  });
  const { field: keyField, fieldState: keyState } = useController({
    control: form.control,
    name: "key",
  });
  const profileName = useWatch({ control: form.control, name: "profileName" });

  useEffect(() => {
    if (!profiles.length) return;
    if (profileName && profiles.includes(profileName)) return;
    form.setValue("profileName", profiles[0], { shouldValidate: true });
  }, [profiles, profileName, form]);

  const applyIMSIishInput = (raw: string) => {
    const { msin, mismatchMsg } = parseIMSIorMSIN(raw, mcc, mnc);
    setImsiMismatch(mismatchMsg);
    if (msin !== null) {
      form.setValue("msin", msin, { shouldValidate: true, shouldTouch: true });
    }
  };

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await createSubscriber(
      accessToken,
      `${mcc}${mnc}${values.msin}`,
      values.key,
      values.sequenceNumber,
      values.profileName,
      values.opc,
      values.description,
    );
  };

  const showMsinError =
    (!!msinState.error && msinState.isTouched) || !!imsiMismatch;
  const showKeyError = !!keyState.error && keyState.isTouched;
  const loadFailed = operatorQuery.isError || profilesQuery.isError;

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Create Subscriber"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to create subscriber"
      submitLabel="Create"
      submittingLabel="Creating..."
    >
      {loadFailed && (
        <Alert severity="error" sx={{ mb: 2 }}>
          Failed to load operator or profile data. Please try again.
        </Alert>
      )}

      <Box sx={{ display: "flex", gap: 2, alignItems: "flex-start" }}>
        <TextField
          fullWidth
          label="IMSI"
          value={msinField.value ?? ""}
          onChange={(event) => applyIMSIishInput(event.target.value)}
          onPaste={(event) => {
            const pasted = event.clipboardData.getData("text");
            if (/\d{12,}/.test(pasted)) {
              event.preventDefault();
              applyIMSIishInput(pasted);
            }
          }}
          onBlur={msinField.onBlur}
          inputRef={msinField.ref}
          error={showMsinError}
          helperText={
            (msinState.isTouched && msinState.error?.message) ||
            imsiMismatch ||
            " "
          }
          margin="normal"
          autoFocus
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">{`${mcc}${mnc}`}</InputAdornment>
              ),
            },
          }}
        />
        <Button
          variant="contained"
          color="primary"
          sx={GENERATE_BUTTON_SX}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() =>
            form.setValue("msin", randomMSIN(mnc.length), {
              shouldValidate: true,
            })
          }
        >
          Generate
        </Button>
      </Box>

      <Box sx={{ display: "flex", gap: 2, alignItems: "flex-start" }}>
        <TextField
          fullWidth
          label="Key"
          value={keyField.value ?? ""}
          onChange={keyField.onChange}
          onBlur={keyField.onBlur}
          inputRef={keyField.ref}
          error={showKeyError}
          helperText={showKeyError ? keyState.error?.message : " "}
          margin="normal"
          sx={{ flex: 1 }}
        />
        <Button
          variant="contained"
          color="primary"
          sx={GENERATE_BUTTON_SX}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() =>
            form.setValue("key", randomKey(), { shouldValidate: true })
          }
        >
          Generate
        </Button>
      </Box>

      <TextControl<FormValues>
        name="sequenceNumber"
        label="Sequence Number"
        helperText="6-byte (12-char) hex string (e.g., 000000000001)"
      />

      <ProfileSelectField
        control={form.control}
        name="profileName"
        profiles={profiles}
      />

      <TextControl<FormValues>
        name="description"
        label="Description (optional)"
        helperText={`A note to identify this subscriber, up to ${MAX_DESCRIPTION_LENGTH} characters`}
        multiline
        minRows={1}
        maxRows={3}
      />

      <FormControlLabel
        control={
          <Checkbox
            checked={customOPC}
            onChange={(event) => {
              setCustomOPC(event.target.checked);
              if (!event.target.checked) {
                form.setValue("opc", "", { shouldValidate: true });
              }
            }}
          />
        }
        label="Provide custom OPC"
      />

      {customOPC && (
        <TextControl<FormValues>
          name="opc"
          label="OPC (optional)"
          helperText="Leave blank to use centrally managed OP"
        />
      )}
    </FormDialog>
  );
};

export default CreateSubscriberModal;
