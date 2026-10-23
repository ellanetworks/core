// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useRef, useState } from "react";
import { useController, useForm, useFormState } from "react-hook-form";
import { useQuery } from "@tanstack/react-query";
import { Link as RouterLink } from "react-router-dom";
import {
  Box,
  Button,
  Chip,
  IconButton,
  Link,
  ListItemText,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  DragIndicator as DragIcon,
  Edit as EditIcon,
} from "@mui/icons-material";
import {
  describeTrigger,
  listIMSApplicationServers,
  setProfileIMSFilterCriteria,
  type IMSApplicationServer,
  type IMSInitialFilterCriterion,
  type IMSInitialFilterCriterionInput,
} from "@/queries/ims";
import { applyPreset, imsServicePresets } from "@/utils/imsServicePresets";
import { sameCriterion } from "@/utils/imsFilterCriteria";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";
import IMSFilterCriterionFormDialog from "@/components/IMSFilterCriterionFormDialog";

interface EditProfileIMSFilterCriteriaModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  profileName: string;
  initialCriteria: IMSInitialFilterCriterion[];
}

interface InMemoryCriterion {
  tempId: string;
  criterion: IMSInitialFilterCriterionInput;
}

interface FormValues {
  criteria: InMemoryCriterion[];
}

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

  const form = useForm<FormValues>({
    defaultValues: {
      criteria: initialCriteria.map(({ priority, ...criterion }) => ({
        tempId: `criterion-${priority}`,
        criterion,
      })),
    },
  });
  const { field } = useController({ control: form.control, name: "criteria" });
  const { isSubmitting } = useFormState({ control: form.control });
  const criteria: InMemoryCriterion[] = field.value ?? [];

  const [formOpen, setFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const nextId = useRef(0);

  const [presetName, setPresetName] = useState("");
  const [serverName, setServerName] = useState("");
  const preset = imsServicePresets.find((p) => p.name === presetName);
  const presetCriteria =
    preset && serverName ? applyPreset(preset, serverName) : [];
  const presetAdded =
    presetCriteria.length > 0 &&
    presetCriteria.every((p) =>
      criteria.some((c) => sameCriterion(c.criterion, p)),
    );

  const dragIndexRef = useRef<number | null>(null);
  const [hoverIndex, setHoverIndex] = useState<number | null>(null);

  const handleDragStart = (index: number) => (event: React.DragEvent) => {
    dragIndexRef.current = index;
    event.dataTransfer.effectAllowed = "move";
    const element = event.currentTarget as HTMLElement;
    event.dataTransfer.setDragImage(
      element,
      element.offsetWidth / 2,
      element.offsetHeight / 2,
    );
  };

  const handleDragOver = (index: number) => (event: React.DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = "move";
    setHoverIndex(index);
  };

  const handleDrop = (targetIndex: number) => (event: React.DragEvent) => {
    event.preventDefault();
    setHoverIndex(null);

    const sourceIndex = dragIndexRef.current;
    dragIndexRef.current = null;
    if (sourceIndex === null || sourceIndex === targetIndex) return;

    const next = [...criteria];
    const [moved] = next.splice(sourceIndex, 1);
    next.splice(targetIndex, 0, moved);
    field.onChange(next);
  };

  const handleDragEnd = () => {
    dragIndexRef.current = null;
    setHoverIndex(null);
  };

  const append = (added: IMSInitialFilterCriterionInput[]) =>
    field.onChange([
      ...criteria,
      ...added.map((criterion) => {
        nextId.current += 1;
        return { tempId: `new-${nextId.current}`, criterion };
      }),
    ]);

  const saveCriterion = (criterion: IMSInitialFilterCriterionInput) => {
    if (editingId) {
      field.onChange(
        criteria.map((c) => (c.tempId === editingId ? { ...c, criterion } : c)),
      );
    } else {
      append([criterion]);
    }
    setFormOpen(false);
  };

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await setProfileIMSFilterCriteria(
      accessToken,
      profileName,
      values.criteria.map((c) => c.criterion),
    );
  };

  return (
    <>
      <FormDialog
        open={open}
        onClose={onClose}
        onSuccess={onSuccess}
        title="Edit IMS Filter Criteria"
        description="The S-CSCF checks the criteria from top to bottom and sends matching requests to their application server. Drag rows to reorder."
        form={form}
        onSubmit={submit}
        errorPrefix="Failed to update IMS filter criteria"
        submitLabel="Save"
        submittingLabel="Saving..."
        maxWidth="md"
      >
        {criteria.length === 0 ? (
          <Typography color="textSecondary" sx={{ p: 2 }}>
            No IMS filter criteria.
          </Typography>
        ) : (
          <Box
            component="ul"
            aria-label="Filter criteria"
            sx={{
              border: 1,
              borderColor: "divider",
              borderRadius: 1,
              listStyle: "none",
              m: 0,
              p: 0,
            }}
          >
            {criteria.map(({ tempId, criterion }, index) => (
              <Box
                key={tempId}
                component="li"
                draggable
                onDragStart={handleDragStart(index)}
                onDragOver={handleDragOver(index)}
                onDrop={handleDrop(index)}
                onDragEnd={handleDragEnd}
                sx={{
                  display: "flex",
                  flexWrap: { xs: "wrap", sm: "nowrap" },
                  alignItems: "center",
                  py: 1,
                  px: 1.5,
                  backgroundColor:
                    hoverIndex === index ? "action.hover" : "transparent",
                  cursor: "grab",
                  transition: "background-color 0.2s",
                  "&:not(:last-child)": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                  },
                }}
              >
                <DragIcon
                  fontSize="small"
                  sx={{ color: "text.secondary", flexShrink: 0, mr: 1 }}
                />
                <Typography
                  variant="body2"
                  sx={{ fontWeight: 600, width: 24, flexShrink: 0 }}
                >
                  {index + 1}
                </Typography>
                <Typography
                  variant="body2"
                  sx={{
                    flex: { xs: 1, sm: "0 0 120px" },
                    minWidth: 0,
                    pr: 1,
                    overflow: { xs: "hidden", sm: "visible" },
                    textOverflow: "ellipsis",
                    whiteSpace: { xs: "nowrap", sm: "normal" },
                    wordBreak: "break-word",
                  }}
                >
                  {criterion.application_server_name}
                </Typography>
                <Typography
                  variant="body2"
                  color="textSecondary"
                  sx={{
                    flex: { xs: "1 1 100%", sm: 1 },
                    order: { xs: 1, sm: 0 },
                    minWidth: 0,
                    wordBreak: "break-word",
                    pr: 1,
                    pl: { xs: 4.5, sm: 0 },
                    pt: { xs: 0.5, sm: 0 },
                  }}
                >
                  {describeTrigger(criterion.trigger)}
                </Typography>
                <Box sx={{ width: { xs: "auto", sm: 96 }, flexShrink: 0 }}>
                  {criterion.default_handling && (
                    <Chip
                      size="small"
                      variant="outlined"
                      label={
                        criterion.default_handling === "SESSION_TERMINATED"
                          ? "Terminate"
                          : "Continue"
                      }
                    />
                  )}
                </Box>
                <Box sx={{ display: "flex", gap: 0.5, flexShrink: 0 }}>
                  <IconButton
                    size="small"
                    color="primary"
                    onClick={() => {
                      setEditingId(tempId);
                      setFormOpen(true);
                    }}
                    aria-label={`Edit criterion ${index + 1}`}
                  >
                    <EditIcon fontSize="small" />
                  </IconButton>
                  <IconButton
                    size="small"
                    color="primary"
                    onClick={() =>
                      field.onChange(
                        criteria.filter((c) => c.tempId !== tempId),
                      )
                    }
                    aria-label={`Delete criterion ${index + 1}`}
                  >
                    <DeleteIcon fontSize="small" />
                  </IconButton>
                </Box>
              </Box>
            ))}
          </Box>
        )}

        {noServers ? (
          <Typography variant="body2" color="textSecondary" sx={{ mt: 2 }}>
            No application servers yet. Add one on the{" "}
            <Link component={RouterLink} to="/operator">
              Operator page
            </Link>{" "}
            first.
          </Typography>
        ) : (
          <>
            <Box sx={{ mt: 2 }}>
              <Button
                variant="outlined"
                size="small"
                startIcon={<AddIcon />}
                onClick={() => {
                  setEditingId(null);
                  setFormOpen(true);
                }}
                disabled={isSubmitting || !serversQuery.isSuccess}
              >
                Add Criterion
              </Button>
            </Box>
            <Typography variant="subtitle2" sx={{ mt: 3 }}>
              Add from preset
            </Typography>
            <Stack
              direction={{ xs: "column", sm: "row" }}
              spacing={{ xs: 0, sm: 2 }}
              sx={{ alignItems: { sm: "flex-start" } }}
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
                      imsServicePresets.find((p) => p.name === value)?.label ??
                      (value as string),
                  },
                }}
              >
                {imsServicePresets.map((p) => (
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
                error={presetAdded}
                helperText={
                  presetAdded ? "This preset is already added." : undefined
                }
              >
                {servers.map((s) => (
                  <MenuItem key={s.name} value={s.name}>
                    {`${s.name} (${s.server_name})`}
                  </MenuItem>
                ))}
              </TextField>
              <Box sx={{ flexShrink: 0, mt: { sm: 3 } }}>
                <Button
                  variant="outlined"
                  startIcon={<AddIcon />}
                  onClick={() => append(presetCriteria)}
                  aria-label="Add preset"
                  disabled={presetCriteria.length === 0 || presetAdded}
                >
                  Add
                </Button>
              </Box>
            </Stack>
          </>
        )}
      </FormDialog>

      {formOpen && (
        <IMSFilterCriterionFormDialog
          open
          onClose={() => setFormOpen(false)}
          onSave={saveCriterion}
          criterion={criteria.find((c) => c.tempId === editingId)?.criterion}
          servers={servers}
        />
      )}
    </>
  );
};

export default EditProfileIMSFilterCriteriaModal;
