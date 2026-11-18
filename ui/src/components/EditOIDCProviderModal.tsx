// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect } from "react";
import {
  Alert,
  Box,
  FormControlLabel,
  Stack,
  Switch,
  Tooltip,
} from "@mui/material";
import { useQueryClient } from "@tanstack/react-query";
import {
  Controller,
  useForm,
  useWatch,
  type FieldErrors,
  type Resolver,
} from "react-hook-form";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import SelectControl from "@/components/form/SelectControl";
import PemControl from "@/components/form/PemControl";
import {
  caCertificateError,
  currentUpdate,
  httpsURLError,
  MAX_DISPLAY_NAME_BYTES,
  MAX_SESSION_LIFETIME_SECONDS,
  MAX_VALUE_BYTES,
  MIN_SESSION_LIFETIME_SECONDS,
  OIDC_QUERY_KEY,
  scopesError,
  splitScopes,
  textError,
  updateOIDC,
  type OIDCSettings,
} from "@/queries/oidc";
import { PRODUCT } from "@/utils/product";

const LIFETIME_UNITS = [
  { label: "days", value: 86400 },
  { label: "hours", value: 3600 },
  { label: "minutes", value: 60 },
  { label: "seconds", value: 1 },
];

const unitsFor = (seconds: number) =>
  LIFETIME_UNITS.filter((u) => u.value >= 60 || seconds % 60 !== 0);

const largestUnit = (seconds: number) =>
  LIFETIME_UNITS.find((u) => seconds % u.value === 0)!.value;

interface FormValues {
  issuer: string;
  caCertificate: string;
  clientID: string;
  clientSecret: string;
  displayName: string;
  scopes: string;
  lifetime: string;
  lifetimeUnit: number;
  trustEmail: boolean;
}

const lifetimeSeconds = (values: FormValues) =>
  /^\d+$/.test(values.lifetime.trim())
    ? Number(values.lifetime.trim()) * values.lifetimeUnit
    : NaN;

const identityChanged = (values: FormValues, settings: OIDCSettings) =>
  values.issuer.trim() !== settings.issuer ||
  values.clientID.trim() !== settings.client_id;

function validate(
  values: FormValues,
  settings: OIDCSettings,
): Partial<Record<keyof FormValues, string>> {
  const enabled = settings.enabled;
  const seconds = lifetimeSeconds(values);
  return {
    displayName: textError(
      values.displayName.trim(),
      MAX_DISPLAY_NAME_BYTES,
      enabled ? "Enter the name to show on the login page." : undefined,
    ),
    issuer: httpsURLError(values.issuer.trim(), {
      missing: enabled ? "Enter the issuer URL." : undefined,
    }),
    clientID: textError(
      values.clientID.trim(),
      MAX_VALUE_BYTES,
      enabled ? "Enter the client ID." : undefined,
    ),
    clientSecret:
      textError(values.clientSecret, MAX_VALUE_BYTES) ??
      (values.clientSecret !== ""
        ? undefined
        : settings.client_secret_set
          ? identityChanged(values, settings)
            ? "Enter the client secret again: the issuer URL or client ID changed."
            : undefined
          : enabled
            ? "Enter the client secret."
            : undefined),
    scopes: scopesError(splitScopes(values.scopes)),
    lifetime:
      !Number.isSafeInteger(seconds) ||
      seconds < MIN_SESSION_LIFETIME_SECONDS ||
      seconds > MAX_SESSION_LIFETIME_SECONDS
        ? "From 5 minutes to 30 days."
        : undefined,
    caCertificate: caCertificateError(values.caCertificate.trim()),
  };
}

interface EditOIDCProviderModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  settings: OIDCSettings;
}

const EditOIDCProviderModal: React.FC<EditOIDCProviderModalProps> = ({
  open,
  onClose,
  onSuccess,
  settings,
}) => {
  const { accessToken } = useAuth();
  const queryClient = useQueryClient();
  const enabled = settings.enabled;

  const resolver: Resolver<FormValues> = (values) => {
    const errors: FieldErrors<FormValues> = {};
    for (const [name, message] of Object.entries(validate(values, settings))) {
      if (message !== undefined) {
        errors[name as keyof FormValues] = { type: "validate", message };
      }
    }
    return Object.keys(errors).length === 0
      ? { values, errors: {} }
      : { values: {}, errors };
  };

  const initialUnit = largestUnit(settings.session_lifetime_seconds);
  const form = useForm<FormValues>({
    mode: "onChange",
    resolver,
    defaultValues: {
      issuer: settings.issuer,
      caCertificate: settings.ca_certificate,
      clientID: settings.client_id,
      clientSecret: "",
      displayName: settings.display_name,
      scopes: settings.scopes.join(" "),
      lifetime: String(settings.session_lifetime_seconds / initialUnit),
      lifetimeUnit: initialUnit,
      trustEmail: settings.trust_unverified_email,
    },
  });

  useEffect(() => {
    void form.trigger();
    return form.subscribe({
      formState: { values: true },
      callback: () => void form.trigger(),
    });
  }, [form]);

  const values = useWatch({ control: form.control }) as FormValues;
  const signsOut = enabled && values.issuer.trim() !== settings.issuer;

  const submit = async (submitted: FormValues) => {
    if (!accessToken) return false;
    const updated = await updateOIDC(accessToken, {
      ...currentUpdate(settings),
      issuer: submitted.issuer.trim(),
      client_id: submitted.clientID.trim(),
      client_secret: submitted.clientSecret,
      display_name: submitted.displayName.trim(),
      scopes: splitScopes(submitted.scopes),
      session_lifetime_seconds: lifetimeSeconds(submitted),
      trust_unverified_email: submitted.trustEmail,
      ca_certificate: submitted.caCertificate.trim(),
    });
    queryClient.setQueryData(OIDC_QUERY_KEY, updated);
    void queryClient.invalidateQueries({ queryKey: ["users"] });
    void queryClient.invalidateQueries({ queryKey: ["user"] });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit Provider"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update single sign-on"
      submitLabel="Update"
      submittingLabel="Updating..."
    >
      <TextControl<FormValues>
        name="displayName"
        label="Display name"
        required={enabled}
        helperText='Shown on the login page: "Log in with …".'
      />
      <TextControl<FormValues>
        name="issuer"
        label="Issuer URL"
        required={enabled}
        helperText="Such as https://idp.example.com/realms/core."
      />
      <PemControl<FormValues>
        name="caCertificate"
        label="CA certificate"
        helperText="Optional. Only if a private CA signs the identity provider's certificate."
        accept=".pem,.crt,.cer"
        minRows={2}
        maxRows={4}
      />
      <TextControl<FormValues>
        name="clientID"
        label="Client ID"
        required={enabled}
      />
      <TextControl<FormValues>
        name="clientSecret"
        label="Client secret"
        type="password"
        autoComplete="new-password"
        showErrorWhileTyping
        helperText={
          settings.client_secret_set
            ? "Leave empty to keep the current secret."
            : undefined
        }
      />
      <TextControl<FormValues>
        name="scopes"
        label="Scopes"
        showErrorWhileTyping
        helperText="Separated by spaces."
      />
      <Stack direction="row" spacing={1} sx={{ alignItems: "start" }}>
        <Box sx={{ flex: 1 }}>
          <TextControl<FormValues>
            name="lifetime"
            label="Session lifetime"
            type="number"
            showErrorWhileTyping
            helperText="After this, users sign in through the identity provider again."
            slotProps={{ htmlInput: { min: 1, step: 1 } }}
          />
        </Box>
        <Box sx={{ width: 140 }}>
          <SelectControl<FormValues, number>
            name="lifetimeUnit"
            label="Unit"
            options={unitsFor(settings.session_lifetime_seconds)}
            numeric
          />
        </Box>
      </Stack>
      <Tooltip
        describeChild
        title={`Anyone who can set their email address at this provider can log in as the ${PRODUCT.name} user with that address, Admins included. Turn this on only if the provider does not say whether addresses are verified, and controls its users' addresses.`}
      >
        <FormControlLabel
          control={
            <Controller
              name="trustEmail"
              control={form.control}
              render={({ field }) => (
                <Switch
                  checked={field.value}
                  onChange={(e) => field.onChange(e.target.checked)}
                  onBlur={field.onBlur}
                />
              )}
            />
          }
          label="Trust email addresses from this provider"
        />
      </Tooltip>
      {signsOut && (
        <Alert severity="warning" sx={{ mt: 2 }}>
          Saving signs out everyone who logged in with single sign-on.
        </Alert>
      )}
    </FormDialog>
  );
};

export default EditOIDCProviderModal;
