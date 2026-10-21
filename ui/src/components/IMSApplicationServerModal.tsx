// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useFieldArray, useForm } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { Box, Button, IconButton, Tooltip, Typography } from "@mui/material";
import { Add as AddIcon, Delete as DeleteIcon } from "@mui/icons-material";
import {
  createIMSApplicationServer,
  isSIPURI,
  updateIMSApplicationServer,
  type IMSApplicationServer,
} from "@/queries/ims";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";

interface IMSApplicationServerModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  applicationServer?: IMSApplicationServer;
}

const sipURI = (required: string) =>
  yup
    .string()
    .default("")
    .test("required", required, (v) => !!v?.trim())
    .test(
      "uri",
      "Must be a SIP URI, for example sip:as.ims.example.org",
      (v) => !v?.trim() || isSIPURI(v),
    );

const schema = yup.object({
  name: yup
    .string()
    .default("")
    .test("required", "Name is required", (v) => !!v?.trim()),
  serverName: sipURI("Server name is required"),
  psis: yup
    .array()
    .of(yup.object({ value: sipURI("Enter an identity or remove the row") }))
    .default([])
    .test("unique", "An identity is listed twice", (rows) => {
      const values = (rows ?? []).map((r) => r.value.trim());
      return new Set(values).size === values.length;
    }),
});

type FormValues = yup.InferType<typeof schema>;

const IMSApplicationServerModal: React.FC<IMSApplicationServerModalProps> = ({
  open,
  onClose,
  onSuccess,
  applicationServer,
}) => {
  const { accessToken } = useAuth();
  const editing = !!applicationServer;
  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: {
      name: applicationServer?.name ?? "",
      serverName: applicationServer?.server_name ?? "",
      psis: (applicationServer?.public_service_identities ?? []).map(
        (value) => ({ value }),
      ),
    },
  });
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: "psis",
  });
  const psisError =
    form.formState.errors.psis?.root?.message ??
    form.formState.errors.psis?.message;

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    const input = {
      server_name: values.serverName.trim(),
      public_service_identities: values.psis.map((p) => p.value.trim()),
    };
    if (applicationServer) {
      await updateIMSApplicationServer(
        accessToken,
        applicationServer.name,
        input,
      );
    } else {
      await createIMSApplicationServer(accessToken, {
        name: values.name.trim(),
        ...input,
      });
    }
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title={editing ? "Edit Application Server" : "Add Application Server"}
      description="A SIP application server that the S-CSCF forwards matching requests to."
      form={form}
      onSubmit={submit}
      errorPrefix={
        editing
          ? "Failed to update application server"
          : "Failed to add application server"
      }
      submitLabel={editing ? "Update" : "Add"}
      submittingLabel={editing ? "Updating..." : "Adding..."}
    >
      <TextControl<FormValues>
        name="name"
        label="Name"
        placeholder="mcptt"
        disabled={editing}
        autoFocus={!editing}
      />
      <TextControl<FormValues>
        name="serverName"
        label="Server Name"
        placeholder="sip:mcptt.ims.example.org"
        helperText="SIP URI the S-CSCF sends requests to."
        autoFocus={editing}
      />
      <Typography variant="subtitle2" sx={{ mt: 2 }}>
        Public Service Identities
      </Typography>
      <Typography variant="body2" color="textSecondary">
        SIP URIs this server hosts. Use !.*! as a wildcard. Optional.
      </Typography>
      {fields.map((field, index) => (
        <Box key={field.id} sx={{ display: "flex", alignItems: "center" }}>
          <TextControl<FormValues>
            name={`psis.${index}.value`}
            label={`Identity ${index + 1}`}
            placeholder="sip:!.*!@mcptt.ims.example.org"
          />
          <Tooltip title="Remove identity" arrow>
            <IconButton
              aria-label={`Remove identity ${index + 1}`}
              onClick={() => remove(index)}
              sx={{ ml: 1 }}
            >
              <DeleteIcon fontSize="small" color="primary" />
            </IconButton>
          </Tooltip>
        </Box>
      ))}
      {psisError && (
        <Typography variant="body2" color="error" sx={{ mt: 1 }}>
          {psisError}
        </Typography>
      )}
      <Button
        size="small"
        startIcon={<AddIcon />}
        onClick={() => append({ value: "" })}
        sx={{ mt: 1 }}
      >
        Add Identity
      </Button>
    </FormDialog>
  );
};

export default IMSApplicationServerModal;
