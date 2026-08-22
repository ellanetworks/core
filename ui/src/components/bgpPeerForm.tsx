// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import {
  Box,
  Button,
  Collapse,
  IconButton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from "@mui/material";
import {
  Delete as DeleteIcon,
  Add as AddIcon,
  Lock as LockIcon,
  ExpandMore as ExpandMoreIcon,
  ExpandLess as ExpandLessIcon,
} from "@mui/icons-material";
import * as yup from "yup";
import { useController } from "react-hook-form";
import type { Control, FieldValues, Path } from "react-hook-form";
import type { BGPImportPrefix, RejectedPrefix } from "@/queries/bgp";
import {
  ipRegex,
  isValidCidr,
  getMaxPrefixLength,
  prefixLength,
} from "@/utils/ip";
import { detectPreset, type ImportPreset } from "@/utils/bgp";
import TextControl from "@/components/form/TextControl";
import NumberControl from "@/components/form/NumberControl";

export const isPrefixEntryInvalid = (entry: BGPImportPrefix): boolean => {
  if (!entry.prefix) return false;
  const minLen = prefixLength(entry.prefix) ?? 0;
  const maxLen = getMaxPrefixLength(entry.prefix);
  return (
    !isValidCidr(entry.prefix) ||
    entry.maxLength < minLen ||
    entry.maxLength > maxLen
  );
};

export const bgpPeerSchema = {
  address: yup
    .string()
    .required("Neighbor address is required")
    .matches(ipRegex, "Must be a valid IP address"),
  remoteAS: yup
    .number()
    .required("Remote AS is required")
    .min(1, "Must be at least 1")
    .max(4294967295, "Must be at most 4294967295"),
  holdTime: yup
    .number()
    .required("Hold time is required")
    .min(3, "Must be at least 3")
    .max(65535, "Must be at most 65535"),
  password: yup.string().default(""),
  description: yup.string().default(""),
  importPrefixes: yup
    .mixed<BGPImportPrefix[]>()
    .required()
    .test(
      "valid-prefixes",
      "Import prefix list is invalid",
      (prefixes) => !(prefixes ?? []).some(isPrefixEntryInvalid),
    ),
};

export const BGPPeerIdentityFields = () => (
  <>
    <TextControl
      name="address"
      label="Neighbor Address"
      placeholder="e.g. 10.0.0.1"
      autoFocus
    />
    <NumberControl
      name="remoteAS"
      label="Remote AS"
      helperText="Autonomous System number of the remote peer"
    />
    <NumberControl
      name="holdTime"
      label="Hold Time"
      helperText={"Seconds before the session is considered down (3–65535)"}
    />
  </>
);

const PRESET_DESCRIPTIONS: Partial<Record<ImportPreset, string>> = {
  none: "All routes from this peer will be rejected.",
  "default-route": "Only the default route (0.0.0.0/0) will be accepted.",
  all: "All routes will be accepted from this peer.",
};

interface ImportPrefixEditorProps<T extends FieldValues> {
  control: Control<T>;
  name: Path<T>;
}

export const ImportPrefixEditor = <T extends FieldValues>({
  control,
  name,
}: ImportPrefixEditorProps<T>) => {
  const { field } = useController({ control, name });
  const prefixes: BGPImportPrefix[] = field.value ?? [];
  const preset = detectPreset(prefixes);

  const setPrefixes = (next: BGPImportPrefix[]) => field.onChange(next);

  const handlePresetChange = (
    _event: React.MouseEvent<HTMLElement>,
    value: ImportPreset | null,
  ) => {
    if (!value) return;
    switch (value) {
      case "none":
        setPrefixes([]);
        break;
      case "default-route":
        setPrefixes([{ prefix: "0.0.0.0/0", maxLength: 0 }]);
        break;
      case "all":
        setPrefixes([{ prefix: "0.0.0.0/0", maxLength: 32 }]);
        break;
      case "custom":
        if (prefixes.length === 0) setPrefixes([{ prefix: "", maxLength: 32 }]);
        break;
    }
  };

  const handlePrefixChange = (
    index: number,
    key: "prefix" | "maxLength",
    value: string | number,
  ) => {
    const next = [...prefixes];
    next[index] = { ...next[index], [key]: value };
    setPrefixes(next);
  };

  return (
    <>
      <Typography variant="subtitle2" sx={{ mt: 3, mb: 1 }}>
        Import Prefix List
      </Typography>
      <Typography variant="body2" color="textSecondary" sx={{ mb: 1 }}>
        Control which routes this peer is allowed to advertise to Ella Core.
      </Typography>

      <ToggleButtonGroup
        value={preset}
        exclusive
        onChange={handlePresetChange}
        size="small"
        sx={{ mb: 2 }}
      >
        <ToggleButton value="none">Deny All</ToggleButton>
        <ToggleButton value="default-route">Default Route Only</ToggleButton>
        <ToggleButton value="all">Accept All</ToggleButton>
        <ToggleButton value="custom">Custom</ToggleButton>
      </ToggleButtonGroup>

      {PRESET_DESCRIPTIONS[preset] && (
        <Typography variant="body2" color="textSecondary">
          {PRESET_DESCRIPTIONS[preset]}
        </Typography>
      )}

      {preset === "custom" && (
        <>
          {prefixes.map((entry, index) => {
            const badPrefix = !!entry.prefix && !isValidCidr(entry.prefix);
            const badLength =
              !!entry.prefix &&
              (entry.maxLength < 0 ||
                entry.maxLength > getMaxPrefixLength(entry.prefix));

            return (
              <Stack
                key={index}
                direction="row"
                spacing={1}
                sx={{ mb: 1, alignItems: "center" }}
              >
                <TextField
                  label="Prefix"
                  value={entry.prefix}
                  onChange={(event) =>
                    handlePrefixChange(index, "prefix", event.target.value)
                  }
                  size="small"
                  error={badPrefix}
                  helperText={
                    badPrefix
                      ? "Must be valid CIDR (e.g., 10.0.0.0/8 or 2001:db8::/32)"
                      : ""
                  }
                  sx={{ flex: 2 }}
                />
                <TextField
                  label="Max Length"
                  type="number"
                  value={entry.maxLength}
                  onChange={(event) =>
                    handlePrefixChange(
                      index,
                      "maxLength",
                      Number(event.target.value),
                    )
                  }
                  size="small"
                  error={badLength}
                  helperText={
                    badLength
                      ? `${prefixLength(entry.prefix) ?? 0}–${getMaxPrefixLength(entry.prefix)}`
                      : ""
                  }
                  sx={{ flex: 1 }}
                />
                <IconButton
                  size="small"
                  onClick={() =>
                    setPrefixes(prefixes.filter((_, i) => i !== index))
                  }
                  color="primary"
                >
                  <DeleteIcon fontSize="small" />
                </IconButton>
              </Stack>
            );
          })}

          <Button
            size="small"
            startIcon={<AddIcon />}
            onClick={() =>
              setPrefixes([...prefixes, { prefix: "", maxLength: 32 }])
            }
            sx={{ mt: 1, mb: 2 }}
          >
            Add Prefix
          </Button>
        </>
      )}
    </>
  );
};

export const RejectedPrefixesPanel = ({
  rejectedPrefixes,
}: {
  rejectedPrefixes: RejectedPrefix[];
}) => {
  const [showRejected, setShowRejected] = useState(false);

  if (rejectedPrefixes.length === 0) return null;

  return (
    <Box sx={{ mt: 1 }}>
      <Button
        size="small"
        onClick={() => setShowRejected((value) => !value)}
        startIcon={<LockIcon fontSize="small" />}
        endIcon={
          showRejected ? (
            <ExpandLessIcon fontSize="small" />
          ) : (
            <ExpandMoreIcon fontSize="small" />
          )
        }
        sx={{
          justifyContent: "flex-start",
          textTransform: "none",
          color: "text.secondary",
        }}
      >
        {rejectedPrefixes.length} rejected{" "}
        {rejectedPrefixes.length === 1 ? "prefix" : "prefixes"} (system)
      </Button>
      <Collapse in={showRejected}>
        <Typography variant="body2" color="textSecondary" sx={{ mb: 1 }}>
          These prefixes are always rejected regardless of import policy.
        </Typography>
        <TableContainer>
          <Table size="small">
            <TableBody>
              {rejectedPrefixes.map((rejected) => (
                <TableRow key={rejected.prefix} sx={{ opacity: 0.7 }}>
                  <TableCell>{rejected.prefix}</TableCell>
                  <TableCell>{rejected.description}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Collapse>
    </Box>
  );
};
