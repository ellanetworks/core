// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useRef, useState } from "react";
import {
  Alert,
  Box,
  Checkbox,
  Divider,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Typography,
} from "@mui/material";
import { DragIndicator as DragIcon } from "@mui/icons-material";
import { useController, useForm, useWatch } from "react-hook-form";
import type { Control } from "react-hook-form";
import { updateOperatorNASSecurity } from "@/queries/operator";
import { useAuth } from "@/contexts/AuthContext";
import FormDialog from "@/components/form/FormDialog";

interface EditOperatorNASSecurityModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    ciphering: string[];
    integrity: string[];
  };
}

interface AlgorithmEntry {
  name: string;
  enabled: boolean;
}

interface FormValues {
  ciphering: AlgorithmEntry[];
  integrity: AlgorithmEntry[];
}

const ALL_CIPHERING = ["NULL", "SNOW3G", "AES"];
const ALL_INTEGRITY = ["NULL", "SNOW3G", "AES"];

const describeAlgorithm = (name: string, kind: string): React.ReactNode => {
  const cipher = kind === "ciphering";
  const fourG = { NULL: "EEA0", SNOW3G: "128-EEA1", AES: "128-EEA2" }[name];
  const fiveG = { NULL: "NEA0", SNOW3G: "128-NEA1", AES: "128-NEA2" }[name];
  const ids =
    fourG && fiveG
      ? ` — ${cipher ? fourG : fourG.replace("EEA", "EIA")} (4G) / ${
          cipher ? fiveG : fiveG.replace("NEA", "NIA")
        } (5G)`
      : "";

  if (name === "NULL") {
    return (
      <>
        NULL{ids}{" "}
        <Box component="span" sx={{ color: "warning.main", fontWeight: 500 }}>
          (no {cipher ? "encryption" : "integrity"})
        </Box>
      </>
    );
  }

  return `${name === "SNOW3G" ? "SNOW 3G" : name}${ids}`;
};

const isNullAlgorithm = (name: string) => name === "NULL";

const CANONICAL_ORDER: Record<string, number> = {
  AES: 0,
  SNOW3G: 1,
  NULL: 2,
};

const buildEntries = (enabled: string[], all: string[]): AlgorithmEntry[] => {
  const entries: AlgorithmEntry[] = enabled.map((name) => ({
    name,
    enabled: true,
  }));
  const disabled = all
    .filter((name) => !enabled.includes(name))
    .sort((a, b) => (CANONICAL_ORDER[a] ?? 0) - (CANONICAL_ORDER[b] ?? 0));
  for (const name of disabled) {
    entries.push({ name, enabled: false });
  }
  return entries;
};

const enabledNames = (list: AlgorithmEntry[]) =>
  list.filter((entry) => entry.enabled).map((entry) => entry.name);

interface AlgorithmListFieldProps {
  control: Control<FormValues>;
  name: keyof FormValues;
  title: string;
}

const AlgorithmListField: React.FC<AlgorithmListFieldProps> = ({
  control,
  name,
  title,
}) => {
  const { field, fieldState } = useController({
    control,
    name,
    rules: {
      validate: (list: AlgorithmEntry[]) =>
        list.some((entry) => entry.enabled) ||
        "At least one algorithm must be enabled.",
    },
  });

  const dragIndexRef = useRef<number | null>(null);
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null);

  const list = field.value;

  const toggle = (index: number) => {
    const next = [...list];
    next[index] = { ...next[index], enabled: !next[index].enabled };
    field.onChange(next);
  };

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
    if (dragIndexRef.current === null) return;
    event.dataTransfer.dropEffect = "move";
    setDragOverIndex(index);
  };

  const handleDrop = (index: number) => (event: React.DragEvent) => {
    event.preventDefault();
    const fromIndex = dragIndexRef.current;
    if (fromIndex === null || fromIndex === index) return;
    const next = [...list];
    const [moved] = next.splice(fromIndex, 1);
    next.splice(index, 0, moved);
    field.onChange(next);
    dragIndexRef.current = null;
    setDragOverIndex(null);
  };

  const handleDragEnd = () => {
    dragIndexRef.current = null;
    setDragOverIndex(null);
  };

  return (
    <Box sx={{ mb: 2 }}>
      <Typography variant="subtitle2" sx={{ mb: 1 }}>
        {title}
      </Typography>
      <List dense disablePadding>
        {list.map((entry, index) => (
          <ListItem
            key={entry.name}
            draggable
            onDragStart={handleDragStart(index)}
            onDragOver={handleDragOver(index)}
            onDrop={handleDrop(index)}
            onDragEnd={handleDragEnd}
            disablePadding
            sx={{
              pl: 0.5,
              pr: 1,
              opacity: entry.enabled ? 1 : 0.5,
              borderTop:
                dragOverIndex === index ? "2px solid" : "2px solid transparent",
              borderColor:
                dragOverIndex === index ? "primary.main" : "transparent",
              transition: "border-color 0.15s ease",
              cursor: "grab",
              "&:active": { cursor: "grabbing" },
              userSelect: "none",
            }}
          >
            <ListItemIcon sx={{ minWidth: 28, color: "text.disabled" }}>
              <DragIcon fontSize="small" />
            </ListItemIcon>
            <ListItemIcon sx={{ minWidth: 36 }}>
              <Checkbox
                edge="start"
                checked={entry.enabled}
                onChange={() => toggle(index)}
                size="small"
                slotProps={{ input: { "aria-label": entry.name } }}
              />
            </ListItemIcon>
            <ListItemText
              primary={describeAlgorithm(entry.name, name)}
              slotProps={{
                primary: {
                  variant: "body2",
                  color: "textPrimary",
                },
              }}
            />
          </ListItem>
        ))}
      </List>
      {fieldState.error && (
        <Alert severity="error" sx={{ mt: 1 }}>
          {fieldState.error.message}
        </Alert>
      )}
    </Box>
  );
};

const EditOperatorNASSecurityModal: React.FC<
  EditOperatorNASSecurityModalProps
> = ({ open, onClose, onSuccess, initialData }) => {
  const { accessToken } = useAuth();

  const form = useForm<FormValues>({
    mode: "onChange",
    values: {
      ciphering: buildEntries(initialData.ciphering, ALL_CIPHERING),
      integrity: buildEntries(initialData.integrity, ALL_INTEGRITY),
    },
  });

  const ciphering = useWatch({ control: form.control, name: "ciphering" });
  const integrity = useWatch({ control: form.control, name: "integrity" });

  const enabledCiphering = enabledNames(ciphering);
  const enabledIntegrity = enabledNames(integrity);

  const nullCipheringPreferred = isNullAlgorithm(enabledCiphering[0]);
  const nullIntegrityPreferred = isNullAlgorithm(enabledIntegrity[0]);
  const nullCipheringOffered = enabledCiphering.some(isNullAlgorithm);
  const nullIntegrityOffered = enabledIntegrity.some(isNullAlgorithm);

  const submit = async (values: FormValues) => {
    if (!accessToken) return false;
    await updateOperatorNASSecurity(
      accessToken,
      enabledNames(values.ciphering),
      enabledNames(values.integrity),
    );
  };

  return (
    <FormDialog
      open={open}
      onClose={onClose}
      onSuccess={onSuccess}
      title="Edit NAS Security Algorithms"
      description="Configure the security algorithms used to protect NAS signaling between the subscriber and Ella Core. The order determines which algorithm Ella Core prefers."
      form={form}
      onSubmit={submit}
      errorPrefix="Failed to update security algorithms"
      submitLabel="Update"
      submittingLabel="Updating..."
    >
      {(nullCipheringPreferred || nullIntegrityPreferred) && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          NULL is the preferred{" "}
          {nullCipheringPreferred && nullIntegrityPreferred
            ? "ciphering and integrity algorithm"
            : nullCipheringPreferred
              ? "ciphering algorithm"
              : "integrity algorithm"}
          , so NAS signaling will normally carry no{" "}
          {nullCipheringPreferred && nullIntegrityPreferred
            ? "encryption or integrity protection"
            : nullCipheringPreferred
              ? "encryption"
              : "integrity protection"}
          . Move another algorithm above NULL to prefer it.
        </Alert>
      )}

      {!nullCipheringPreferred &&
        !nullIntegrityPreferred &&
        (nullCipheringOffered || nullIntegrityOffered) && (
          <Alert severity="warning" sx={{ mb: 2 }}>
            NULL is enabled as a fallback. Subscribers that offer no stronger
            algorithm will use unprotected NAS signaling.
          </Alert>
        )}

      <AlgorithmListField
        control={form.control}
        name="ciphering"
        title="Ciphering Preference"
      />
      <Divider sx={{ my: 1 }} />
      <AlgorithmListField
        control={form.control}
        name="integrity"
        title="Integrity Preference"
      />
    </FormDialog>
  );
};

export default EditOperatorNASSecurityModal;
