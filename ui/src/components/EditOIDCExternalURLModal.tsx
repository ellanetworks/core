// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useForm, type FieldErrors, type Resolver } from "react-hook-form";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import {
  currentUpdate,
  httpsURLError,
  OIDC_QUERY_KEY,
  updateOIDC,
  type OIDCSettings,
} from "@/queries/oidc";
import { PRODUCT } from "@/utils/product";

interface FormValues {
  externalURL: string;
}

const origin = (externalURL: string) => externalURL.trim().replace(/\/$/, "");

interface EditOIDCExternalURLModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  settings: OIDCSettings;
}

const EditOIDCExternalURLModal: React.FC<EditOIDCExternalURLModalProps> = ({
  open,
  onClose,
  onSuccess,
  settings,
}) => {
  const { accessToken } = useAuth();
  const queryClient = useQueryClient();

  const resolver: Resolver<FormValues> = (values) => {
    const errors: FieldErrors<FormValues> = {};
    const message = httpsURLError(values.externalURL.trim(), {
      origin: true,
      missing: settings.enabled ? "Enter the external URL." : undefined,
    });
    if (message !== undefined) {
      errors.externalURL = { type: "validate", message };
    }
    return message === undefined
      ? { values, errors: {} }
      : { values: {}, errors };
  };

  const form = useForm<FormValues>({
    mode: "onChange",
    resolver,
    defaultValues: {
      externalURL:
        settings.external_url ||
        (window.location.protocol === "https:" ? window.location.origin : ""),
    },
  });

  useEffect(() => {
    void form.trigger();
  }, [form]);

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    const updated = await updateOIDC(accessToken, {
      ...currentUpdate(settings),
      external_url: origin(values.externalURL),
    });
    queryClient.setQueryData(OIDC_QUERY_KEY, updated);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit External URL"
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update single sign-on"
      submitLabel="Update"
      submittingLabel="Updating..."
    >
      <TextControl<FormValues>
        name="externalURL"
        label="External URL"
        required={settings.enabled}
        helperText={`The https address users open ${PRODUCT.name} at.`}
      />
    </FormDialog>
  );
};

export default EditOIDCExternalURLModal;
