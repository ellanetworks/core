// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { useForm, useWatch } from "react-hook-form";
import { yupResolver } from "@hookform/resolvers/yup";
import * as yup from "yup";
import { useQuery } from "@tanstack/react-query";
import { Link as RouterLink } from "react-router-dom";
import {
  Box,
  Button,
  Link,
  ListItemText,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import { Add as AddIcon } from "@mui/icons-material";
import {
  listIMSApplicationServers,
  setProfileIMSFilterCriteria,
  type IMSApplicationServer,
  type IMSInitialFilterCriterion,
  type IMSInitialFilterCriterionInput,
} from "@/queries/ims";
import { applyPreset, imsServicePresets } from "@/utils/imsServicePresets";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import TextControl from "@/components/form/TextControl";

interface EditProfileIMSFilterCriteriaModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  profileName: string;
  initialCriteria: IMSInitialFilterCriterion[];
}

const formatCriteria = (criteria: IMSInitialFilterCriterionInput[]) =>
  JSON.stringify(criteria, null, 2);

const withoutPriority = (criteria: IMSInitialFilterCriterion[]) =>
  criteria.map(({ priority: _priority, ...rest }) => rest);

const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null && !Array.isArray(value);

const parseCriteria = (
  text: string,
): { criteria?: IMSInitialFilterCriterionInput[]; error?: string } => {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return { error: "Must be valid JSON" };
  }
  if (!Array.isArray(parsed)) return { error: "Must be a JSON array" };
  if (!parsed.every(isObject))
    return { error: "Each filter criterion must be a JSON object" };
  const missing = parsed.findIndex(
    (c) =>
      typeof c.application_server_name !== "string" ||
      !c.application_server_name.trim(),
  );
  if (missing !== -1)
    return {
      error: `Filter criterion ${missing + 1} needs an application_server_name`,
    };
  return { criteria: parsed as IMSInitialFilterCriterionInput[] };
};

const schema = yup.object({
  criteria: yup
    .string()
    .default("")
    .test("criteria", (v, ctx) => {
      const { error } = parseCriteria(v ?? "");
      return error ? ctx.createError({ message: error }) : true;
    }),
});

type FormValues = yup.InferType<typeof schema>;

const EditProfileIMSFilterCriteriaModal: React.FC<
  EditProfileIMSFilterCriteriaModalProps
> = ({ open, onClose, onSuccess, profileName, initialCriteria }) => {
  const { accessToken, authReady } = useAuth();

  const serversQuery = useQuery<IMSApplicationServer[]>({
    queryKey: ["ims-application-servers"],
    enabled: authReady && !!accessToken,
    queryFn: () => listIMSApplicationServers(accessToken!),
  });
  const servers = serversQuery.data ?? [];
  const noServers = serversQuery.isSuccess && servers.length === 0;

  const presets = imsServicePresets;
  const [presetName, setPresetName] = useState("");
  const [serverName, setServerName] = useState("");

  const form = useForm<FormValues>({
    mode: "onTouched",
    resolver: yupResolver(schema),
    defaultValues: {
      criteria: formatCriteria(withoutPriority(initialCriteria)),
    },
  });
  const criteriaText = useWatch({ control: form.control, name: "criteria" });
  const current = criteriaText.trim()
    ? parseCriteria(criteriaText).criteria
    : [];

  const addPreset = () => {
    const preset = presets.find((p) => p.name === presetName);
    if (!preset || !serverName || !current) return;
    form.setValue(
      "criteria",
      formatCriteria([...current, ...applyPreset(preset, serverName)]),
      { shouldDirty: true, shouldTouch: true, shouldValidate: true },
    );
  };

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    const { criteria } = parseCriteria(values.criteria);
    if (!criteria) return false;
    await setProfileIMSFilterCriteria(accessToken, profileName, criteria);
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit IMS Filter Criteria"
      description="Sends requests that match a filter criterion to its application server. Earlier criteria get higher priority."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update IMS filter criteria"
      submitLabel="Save"
      submittingLabel="Saving..."
      maxWidth="md"
    >
      <Typography variant="subtitle2">Add from preset</Typography>
      {noServers ? (
        <Typography variant="body2" color="textSecondary" sx={{ mt: 1 }}>
          No application servers yet. Add one on the{" "}
          <Link component={RouterLink} to="/operator">
            Operator page
          </Link>{" "}
          first.
        </Typography>
      ) : (
        <Stack
          direction={{ xs: "column", sm: "row" }}
          spacing={{ xs: 0, sm: 2 }}
          sx={{ alignItems: { sm: "center" } }}
        >
          <TextField
            select
            fullWidth
            margin="normal"
            label="Preset"
            value={presetName}
            onChange={(e) => setPresetName(e.target.value)}
            slotProps={{
              select: {
                renderValue: (value) =>
                  presets.find((p) => p.name === value)?.label ??
                  (value as string),
              },
            }}
          >
            {presets.map((p) => (
              <MenuItem key={p.name} value={p.name}>
                <ListItemText primary={p.label} secondary={p.description} />
              </MenuItem>
            ))}
          </TextField>
          <TextField
            select
            fullWidth
            margin="normal"
            label="Application Server"
            value={serverName}
            onChange={(e) => setServerName(e.target.value)}
          >
            {servers.map((s) => (
              <MenuItem key={s.name} value={s.name}>
                {`${s.name} (${s.server_name})`}
              </MenuItem>
            ))}
          </TextField>
          <Box sx={{ flexShrink: 0 }}>
            <Button
              variant="outlined"
              startIcon={<AddIcon />}
              onClick={addPreset}
              aria-label="Add preset"
              disabled={!presetName || !serverName || !current}
            >
              Add
            </Button>
          </Box>
        </Stack>
      )}
      <TextControl<FormValues>
        name="criteria"
        label="Filter Criteria (JSON)"
        multiline
        minRows={8}
        maxRows={20}
        helperText="JSON array of initial filter criteria (TS 29.228), in priority order. Each needs an application_server_name. Use [] to clear them."
        sx={{ "& textarea": { fontFamily: "monospace", fontSize: 13 } }}
      />
    </FormDialog>
  );
};

export default EditProfileIMSFilterCriteriaModal;
