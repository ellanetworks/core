// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Controller, useForm } from "react-hook-form";
import { useQuery } from "@tanstack/react-query";
import { Link as RouterLink } from "react-router-dom";
import { Link, MenuItem, TextField, Typography } from "@mui/material";
import {
  imsRegistrationSetLabels,
  imsRegistrationSets,
  listIMSServiceProfiles,
  setProfileIMSServiceProfiles,
  type IMSRegistrationSet,
  type IMSServiceProfile,
  type ProfileIMSServiceProfiles,
} from "@/queries/ims";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";

interface EditProfileIMSServiceProfilesModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  profileName: string;
  initialData: ProfileIMSServiceProfiles;
}

type FormValues = Record<IMSRegistrationSet, string>;

const helperTexts: Record<IMSRegistrationSet, string> = {
  mmtel: "The subscriber's sip:+MSISDN and tel:+MSISDN identities.",
  mcptt: "The subscriber's sip:mcptt-MSISDN identity.",
};

const EditProfileIMSServiceProfilesModal: React.FC<
  EditProfileIMSServiceProfilesModalProps
> = ({ open, onClose, onSuccess, profileName, initialData }) => {
  const { accessToken, authReady } = useAuth();

  const serviceProfilesQuery = useQuery<IMSServiceProfile[]>({
    queryKey: ["ims-service-profiles"],
    enabled: authReady && !!accessToken,
    queryFn: () => listIMSServiceProfiles(accessToken!),
  });
  const names = [
    ...new Set(
      [
        ...(serviceProfilesQuery.data ?? []).map((sp) => sp.name),
        initialData.mmtel,
        initialData.mcptt,
      ].filter((name): name is string => !!name),
    ),
  ];

  const form = useForm<FormValues>({
    defaultValues: {
      mmtel: initialData.mmtel ?? "",
      mcptt: initialData.mcptt ?? "",
    },
  });

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await setProfileIMSServiceProfiles(accessToken, profileName, {
      mmtel: values.mmtel || null,
      mcptt: values.mcptt || null,
    });
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit IMS Service Profiles"
      description="Each implicit registration set gets the filter criteria of its service profile."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update IMS service profiles"
      submitLabel="Save"
      submittingLabel="Saving..."
    >
      {serviceProfilesQuery.isSuccess && names.length === 0 && (
        <Typography variant="body2" color="textSecondary" sx={{ mt: 1 }}>
          No service profiles yet. Add one on the{" "}
          <Link component={RouterLink} to="/operator">
            Operator page
          </Link>{" "}
          first.
        </Typography>
      )}
      {imsRegistrationSets.map((set) => (
        <Controller
          key={set}
          name={set}
          control={form.control}
          render={({ field }) => (
            <TextField
              {...field}
              select
              fullWidth
              margin="normal"
              label={imsRegistrationSetLabels[set]}
              helperText={helperTexts[set]}
            >
              <MenuItem value="">
                <em>None</em>
              </MenuItem>
              {names.map((name) => (
                <MenuItem key={name} value={name}>
                  {name}
                </MenuItem>
              ))}
            </TextField>
          )}
        />
      ))}
    </FormDialog>
  );
};

export default EditProfileIMSServiceProfilesModal;
