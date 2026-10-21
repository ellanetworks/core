// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { FormControlLabel, Radio, RadioGroup, Typography } from "@mui/material";
import {
  createIMSApplicationServer,
  createIMSService,
  isSIPURI,
  listIMSApplicationServers,
  updateIMSService,
  type IMSApplicationServer,
  type IMSService,
} from "@/queries/ims";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import SelectControl from "@/components/form/SelectControl";

interface MCPTTServiceModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  service?: IMSService;
  homeDomain?: string;
}

const SIP_URI_MESSAGE = "Must be a SIP URI, for example sip:as.ims.example.org";

const hasText = (v: string | undefined) => !!v?.trim();

const schema = yup.object({
  name: yup.string().default("").test("required", "Name is required", hasText),
  asMode: yup.string().oneOf(["existing", "new"]).default("existing"),
  asName: yup
    .string()
    .default("")
    .when("asMode", {
      is: "existing",
      then: (s) => s.test("required", "Pick an application server", hasText),
    }),
  newName: yup
    .string()
    .default("")
    .when("asMode", {
      is: "new",
      then: (s) =>
        s.test("required", "Application server name is required", hasText),
    }),
  newServerName: yup
    .string()
    .default("")
    .when("asMode", {
      is: "new",
      then: (s) =>
        s
          .test("required", "Server name is required", hasText)
          .test("uri", SIP_URI_MESSAGE, (v) => !v?.trim() || isSIPURI(v)),
    }),
  newPSI: yup
    .string()
    .default("")
    .when("asMode", {
      is: "new",
      then: (s) =>
        s.test("uri", SIP_URI_MESSAGE, (v) => !v?.trim() || isSIPURI(v)),
    }),
});

type FormValues = yup.InferType<typeof schema>;

const MCPTTServiceModal: React.FC<MCPTTServiceModalProps> = ({
  open,
  onClose,
  onSuccess,
  service,
  homeDomain,
}) => {
  const { accessToken, authReady } = useAuth();
  const queryClient = useQueryClient();
  const editing = !!service;

  const serversQuery = useQuery<IMSApplicationServer[]>({
    queryKey: ["ims-application-servers"],
    enabled: authReady && !!accessToken,
    queryFn: () => listIMSApplicationServers(accessToken!),
  });
  const servers = serversQuery.data ?? [];
  const noServers = serversQuery.isSuccess && servers.length === 0;

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    resetOptions: { keepDirtyValues: true },
    values: {
      name: service?.name ?? "",
      asMode: noServers && !editing ? "new" : "existing",
      asName: service?.application_server_name ?? "",
      newName: "",
      newServerName: homeDomain ? `sip:mcptt.${homeDomain}` : "",
      newPSI: homeDomain ? `sip:!.*!@mcptt.${homeDomain}` : "",
    },
  });
  const asMode = useWatch({ control: form.control, name: "asMode" });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    let applicationServerName = values.asName;
    if (values.asMode === "new") {
      applicationServerName = values.newName.trim();
      const psi = values.newPSI.trim();
      await createIMSApplicationServer(accessToken, {
        name: applicationServerName,
        server_name: values.newServerName.trim(),
        public_service_identities: psi ? [psi] : [],
      });
      await queryClient.invalidateQueries({
        queryKey: ["ims-application-servers"],
      });
      form.setValue("asMode", "existing", { shouldDirty: true });
      form.setValue("asName", applicationServerName, {
        shouldDirty: true,
      });
    }
    if (service) {
      await updateIMSService(accessToken, service.name, {
        kind: "mcptt",
        application_server_name: applicationServerName,
      });
    } else {
      await createIMSService(accessToken, {
        name: values.name.trim(),
        kind: "mcptt",
        application_server_name: applicationServerName,
      });
    }
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title={editing ? "Edit MCPTT Service" : "Add MCPTT Service"}
      description="Routes MCPTT registrations and calls to an application server."
      form={form}
      onSubmit={submit}
      errorPrefix={
        editing ? "Failed to update service" : "Failed to add service"
      }
      submitLabel={editing ? "Update" : "Add"}
      submittingLabel={editing ? "Updating..." : "Adding..."}
    >
      <TextControl<FormValues>
        name="name"
        label="Service Name"
        placeholder="mcptt"
        disabled={editing}
        autoFocus={!editing}
      />
      <Typography variant="subtitle2" sx={{ mt: 2 }}>
        Application Server
      </Typography>
      <Controller
        control={form.control}
        name="asMode"
        render={({ field }) => (
          <RadioGroup
            row
            value={field.value}
            onChange={(e) => {
              field.onChange(e.target.value);
              void form.trigger();
            }}
          >
            <FormControlLabel
              value="existing"
              control={<Radio />}
              label="Use existing"
              disabled={noServers}
            />
            <FormControlLabel
              value="new"
              control={<Radio />}
              label="Create new"
            />
          </RadioGroup>
        )}
      />
      {asMode === "existing" ? (
        <SelectControl<FormValues, string>
          name="asName"
          label="Application Server"
          options={servers.map((s) => ({
            value: s.name,
            label: `${s.name} (${s.server_name})`,
          }))}
        />
      ) : (
        <>
          <TextControl<FormValues>
            name="newName"
            label="Application Server Name"
            placeholder="mcptt"
          />
          <TextControl<FormValues>
            name="newServerName"
            label="Server Name"
            placeholder="sip:mcptt.ims.example.org"
            helperText="SIP URI the S-CSCF sends requests to."
          />
          <TextControl<FormValues>
            name="newPSI"
            label="Public Service Identity"
            placeholder="sip:!.*!@mcptt.ims.example.org"
            helperText="Optional. SIP URI the server hosts. Use !.*! as a wildcard."
          />
        </>
      )}
    </FormDialog>
  );
};

export default MCPTTServiceModal;
