// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useFieldArray, useForm, useWatch } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { Box, Button, IconButton, Tooltip, Typography } from "@mui/material";
import { Add as AddIcon, Delete as DeleteIcon } from "@mui/icons-material";
import {
  DEFAULT_SMSC_PORT,
  MAX_SMSC_NODES,
  MAX_SMSC_SERVICE_CENTRES,
  createSMSC,
  updateSMSC,
  type SMSC,
} from "@/queries/operator";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";
import NumberControl from "@/components/form/NumberControl";
import { isHostAddress, isHostName } from "@/utils/ip";
import { msisdnRegex } from "@/components/subscriberIdentity";

interface SMSCModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  smsc?: SMSC;
}

const diameterIdentityRegex =
  /^(?=.{1,255}$)[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?(?:\.[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?)+$/;

export const parseServiceCentres = (value: string): string[] =>
  value
    .split(/[\s,]+/)
    .map((v) => v.trim())
    .filter(Boolean);

const nodeSchema = yup.object({
  address: yup
    .string()
    .default("")
    .test("required", "Address is required", (v) => !!v?.trim())
    .test(
      "address",
      "Must be an IPv4 or IPv6 address or a host name",
      (v) => !v?.trim() || isHostAddress(v.trim()) || isHostName(v.trim()),
    ),
  port: yup
    .number()
    .typeError("Port is required")
    .integer("Must be a whole number")
    .min(1, "Must be 1 to 65535")
    .max(65535, "Must be 1 to 65535")
    .required("Port is required"),
  diameterIdentity: yup
    .string()
    .default("")
    .test(
      "diameter-identity",
      "Must be a fully qualified domain name",
      (v) => !v?.trim() || diameterIdentityRegex.test(v.trim()),
    ),
});

type NodeRow = {
  address?: string;
  port?: number | null;
  diameterIdentity?: string;
};

const duplicateNodeMessage = (rows: NodeRow[]): string | undefined => {
  const endpoints = rows
    .filter((r) => (r.address ?? "").trim())
    .map((r) => `${(r.address ?? "").trim()}|${r.port}`);
  if (new Set(endpoints).size !== endpoints.length) {
    return "Two nodes have the same address and port";
  }
  const ids = rows
    .map((r) => (r.diameterIdentity ?? "").trim().toLowerCase())
    .filter(Boolean);
  if (new Set(ids).size !== ids.length) {
    return "Two nodes have the same Diameter identity";
  }
  return undefined;
};

const schema = yup.object({
  name: yup
    .string()
    .default("")
    .test("required", "Name is required", (v) => !!v?.trim())
    .max(255, "Name must be at most 255 characters"),
  serviceCentres: yup
    .string()
    .default("")
    .test(
      "required",
      "At least one number is required",
      (v) => parseServiceCentres(v ?? "").length > 0,
    )
    .test(
      "max",
      `At most ${MAX_SMSC_SERVICE_CENTRES} numbers`,
      (v) => parseServiceCentres(v ?? "").length <= MAX_SMSC_SERVICE_CENTRES,
    )
    .test(
      "format",
      "Numbers must be E.164 numbers, for example +15550000000",
      (v) => parseServiceCentres(v ?? "").every((n) => msisdnRegex.test(n)),
    )
    .test("unique", "A number is listed twice", (v) => {
      const numbers = parseServiceCentres(v ?? "");
      return new Set(numbers).size === numbers.length;
    }),
  nodes: yup
    .array()
    .of(nodeSchema)
    .default([])
    .min(1, "At least one node is required")
    .max(MAX_SMSC_NODES, `At most ${MAX_SMSC_NODES} nodes`)
    .test("unique", (rows, ctx) => {
      const message = duplicateNodeMessage(rows ?? []);
      return message ? ctx.createError({ message }) : true;
    }),
});

type FormValues = yup.InferType<typeof schema>;

const emptyNode = () => ({
  address: "",
  port: DEFAULT_SMSC_PORT,
  diameterIdentity: "",
});

const SMSCModal: React.FC<SMSCModalProps> = ({
  open,
  onClose,
  onSuccess,
  smsc,
}) => {
  const { accessToken } = useAuth();
  const editing = !!smsc;
  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    values: {
      name: smsc?.name ?? "",
      serviceCentres: (smsc?.serviceCentres ?? []).join(", "),
      nodes: smsc
        ? smsc.nodes.map((n) => ({
            address: n.address,
            port: n.port,
            diameterIdentity: n.diameterIdentity,
          }))
        : [emptyNode()],
    },
  });
  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: "nodes",
  });
  const nodes = useWatch({ control: form.control, name: "nodes" });
  const nodesError = duplicateNodeMessage(nodes ?? []);

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    const input = {
      serviceCentres: parseServiceCentres(values.serviceCentres),
      nodes: values.nodes.map((n) => ({
        diameterIdentity: n.diameterIdentity.trim(),
        address: n.address.trim(),
        port: n.port,
      })),
    };
    if (smsc) {
      await updateSMSC(accessToken, smsc.name, input);
    } else {
      await createSMSC(accessToken, { name: values.name.trim(), ...input });
    }
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title={editing ? "Edit SMSC" : "Add SMSC"}
      form={form}
      onSubmit={submit}
      errorPrefix={editing ? "Failed to update SMSC" : "Failed to add SMSC"}
      submitLabel={editing ? "Update" : "Add"}
      submittingLabel={editing ? "Updating..." : "Adding..."}
      maxWidth="md"
    >
      <TextControl<FormValues>
        name="name"
        label="Name"
        placeholder="main"
        disabled={editing}
        autoFocus={!editing}
      />
      <TextControl<FormValues>
        name="serviceCentres"
        label="Numbers"
        placeholder="+15550000000"
        helperText="E.164 numbers phones use to reach this SMSC, set as the SMSC number on their SIM. Separate with commas."
        autoFocus={editing}
      />
      <Typography variant="subtitle2" sx={{ mt: 2 }}>
        Nodes
      </Typography>
      {fields.map((field, index) => (
        <Box
          key={field.id}
          sx={{ display: "flex", alignItems: "flex-start", gap: 1 }}
        >
          <Box sx={{ flex: 2, minWidth: 0 }}>
            <TextControl<FormValues>
              name={`nodes.${index}.address`}
              label="Address"
              placeholder="192.0.2.10"
              slotProps={{
                htmlInput: { "aria-label": `Node ${index + 1} address` },
              }}
            />
          </Box>
          <Box sx={{ width: 110, flexShrink: 0 }}>
            <NumberControl<FormValues>
              name={`nodes.${index}.port`}
              label="Port"
              min={1}
              max={65535}
              slotProps={{
                htmlInput: { "aria-label": `Node ${index + 1} port` },
              }}
            />
          </Box>
          <Box sx={{ flex: 3, minWidth: 0 }}>
            <TextControl<FormValues>
              name={`nodes.${index}.diameterIdentity`}
              label="Diameter Identity"
              placeholder="smsc1.example.org"
              slotProps={{
                htmlInput: {
                  "aria-label": `Node ${index + 1} Diameter identity`,
                },
              }}
            />
          </Box>
          <Tooltip title="Remove node" arrow>
            <span>
              <IconButton
                aria-label={`Remove node ${index + 1}`}
                onClick={() => remove(index)}
                disabled={fields.length === 1}
                sx={{ mt: 3 }}
              >
                <DeleteIcon
                  fontSize="small"
                  color={fields.length === 1 ? "disabled" : "primary"}
                />
              </IconButton>
            </span>
          </Tooltip>
        </Box>
      ))}
      {nodesError && (
        <Typography variant="body2" color="error" sx={{ mt: 1 }}>
          {nodesError}
        </Typography>
      )}
      <Button
        size="small"
        startIcon={<AddIcon />}
        onClick={() => append(emptyNode())}
        disabled={fields.length >= MAX_SMSC_NODES}
        sx={{ mt: 1 }}
      >
        Add Node
      </Button>
    </FormDialog>
  );
};

export default SMSCModal;
