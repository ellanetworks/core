// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useState } from "react";
import {
  Box,
  Button,
  MenuItem,
  MenuList,
  Popover,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import AccessTimeIcon from "@mui/icons-material/AccessTime";
import ArrowDropDownIcon from "@mui/icons-material/ArrowDropDown";
import { formatDateTime } from "@/utils/formatters";
import { startOfLocalDay } from "@/utils/dates";

export const CUSTOM_RANGE = "custom";
export const ANY_RANGE = "any";

export type TimeRangeValue = {
  preset: string;
  from: string;
  to: string;
};

export const EMPTY_TIME_RANGE: TimeRangeValue = {
  preset: ANY_RANGE,
  from: "",
  to: "",
};

export type TimeRangeFilter = {
  relative?: string;
  from?: string;
  to?: string;
};

export type RelativeRange = {
  value: string;
  label: string;
  ms: number;
  endMs?: number;
  anchor?: "day";
};

const DAY_MS = 24 * 60 * 60_000;

export const RELATIVE_RANGES: RelativeRange[] = [
  { value: "5m", label: "Last 5 minutes", ms: 5 * 60_000 },
  { value: "15m", label: "Last 15 minutes", ms: 15 * 60_000 },
  { value: "1h", label: "Last 1 hour", ms: 60 * 60_000 },
  { value: "6h", label: "Last 6 hours", ms: 6 * 60 * 60_000 },
  { value: "24h", label: "Last 24 hours", ms: DAY_MS },
  { value: "7d", label: "Last 7 days", ms: 7 * DAY_MS },
];

export const AUDIT_RANGES: RelativeRange[] = [
  { value: "15m", label: "Last 15 minutes", ms: 15 * 60_000 },
  { value: "1h", label: "Last 1 hour", ms: 60 * 60_000 },
  { value: "6h", label: "Last 6 hours", ms: 6 * 60 * 60_000 },
  { value: "24h", label: "Last 24 hours", ms: DAY_MS },
  { value: "7d", label: "Last 7 days", ms: 7 * DAY_MS },
  { value: "30d", label: "Last 30 days", ms: 30 * DAY_MS },
  { value: "90d", label: "Last 90 days", ms: 90 * DAY_MS },
];

export const DAILY_RANGES: RelativeRange[] = [
  { value: "today", label: "Today", ms: 0, endMs: 0, anchor: "day" },
  {
    value: "yesterday",
    label: "Yesterday",
    ms: DAY_MS,
    endMs: DAY_MS,
    anchor: "day",
  },
  {
    value: "7d",
    label: "Last 7 days",
    ms: 6 * DAY_MS,
    endMs: 0,
    anchor: "day",
  },
  {
    value: "30d",
    label: "Last 30 days",
    ms: 29 * DAY_MS,
    endMs: 0,
    anchor: "day",
  },
  {
    value: "90d",
    label: "Last 90 days",
    ms: 89 * DAY_MS,
    endMs: 0,
    anchor: "day",
  },
];

const findRange = (
  value: string,
  ranges: RelativeRange[] = RELATIVE_RANGES,
): RelativeRange | undefined =>
  ranges.find((range) => range.value === value) ??
  [...RELATIVE_RANGES, ...DAILY_RANGES].find((range) => range.value === value);

export const toInstant = (value: string): string => {
  if (!value) return "";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? "" : parsed.toISOString();
};

const MESSAGES = {
  invalid: "Enter a valid date and time.",
  order: "The To timestamp must be on or after the From timestamp.",
  required: "Enter a date and time.",
};

export type TimeRangeFieldErrors = { from?: string; to?: string };

export const isValidBound = (value: string): boolean =>
  Boolean(value) && !Number.isNaN(new Date(value).getTime());

export const timeRangeFieldErrors = (
  value: TimeRangeValue,
  required = false,
): TimeRangeFieldErrors => {
  if (value.preset !== CUSTOM_RANGE) return {};
  const errors: TimeRangeFieldErrors = {};
  if (!isValidBound(value.from) && (value.from || required)) {
    errors.from = value.from ? MESSAGES.invalid : MESSAGES.required;
  }
  if (!isValidBound(value.to) && (value.to || required)) {
    errors.to = value.to ? MESSAGES.invalid : MESSAGES.required;
  }
  if (errors.from || errors.to) return errors;
  const fromIso = toInstant(value.from);
  const toIso = toInstant(value.to);
  if (fromIso && toIso && fromIso > toIso) {
    errors.from = MESSAGES.order;
    errors.to = MESSAGES.order;
  }
  return errors;
};

export const timeRangeError = (value: TimeRangeValue): string => {
  const errors = timeRangeFieldErrors(value);
  return errors.from ?? errors.to ?? "";
};

export const timeRangeFilter = (value: TimeRangeValue): TimeRangeFilter => {
  if (value.preset === CUSTOM_RANGE) {
    const filter: TimeRangeFilter = {};
    const from = toInstant(value.from);
    const to = toInstant(value.to);
    if (from) filter.from = from;
    if (to) filter.to = to;
    return filter;
  }
  if (!value.preset || value.preset === ANY_RANGE) return {};
  return { relative: value.preset };
};

export const toDateInputValue = (stampValue: string | undefined): string => {
  if (!stampValue) return "";
  const parsed = new Date(stampValue);
  if (Number.isNaN(parsed.getTime())) return "";
  return parsed.toISOString().slice(0, 10);
};

export const toLastDateInputValue = (
  stampValue: string | undefined,
): string => {
  if (!stampValue) return "";
  const parsed = new Date(stampValue);
  if (Number.isNaN(parsed.getTime())) return "";
  return toDateInputValue(new Date(parsed.getTime() - DAY_MS).toISOString());
};

export const fromLastDateInputValue = (value: string): string => {
  if (!value) return "";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "";
  return new Date(parsed.getTime() + DAY_MS).toISOString();
};

export const toInputValue = (stampValue: string | undefined): string => {
  if (!stampValue) return "";
  const parsed = new Date(stampValue);
  if (Number.isNaN(parsed.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(
    parsed.getDate(),
  )}T${pad(parsed.getHours())}:${pad(parsed.getMinutes())}`;
};

export const resolveTimeRangeFilter = (
  filter: TimeRangeFilter,
  options?: { ranges?: RelativeRange[] },
): { from?: string; to?: string } => {
  if (!filter.relative) {
    const resolved: { from?: string; to?: string } = {};
    if (filter.from) resolved.from = filter.from;
    if (filter.to) resolved.to = filter.to;
    return resolved;
  }
  const range = findRange(filter.relative, options?.ranges);
  if (!range) return {};
  if (range.anchor === "day") {
    const from = startOfLocalDay(range.ms / DAY_MS);
    const resolved: { from?: string; to?: string } = {
      from: from.toISOString(),
    };
    if (range.endMs !== undefined) {
      const to = startOfLocalDay(range.endMs / DAY_MS - 1);
      resolved.to = to.toISOString();
    }
    return resolved;
  }
  return { from: new Date(Date.now() - range.ms).toISOString() };
};

export const timeRangeParams = (
  filter: TimeRangeFilter,
  options?: { ranges?: RelativeRange[] },
): { start?: string; end?: string } => {
  const { from, to } = resolveTimeRangeFilter(filter, options);
  const params: { start?: string; end?: string } = {};
  if (from) params.start = from;
  if (to) params.end = to;
  return params;
};

export const timeRangeLabel = (
  value: TimeRangeValue,
  options?: { ranges?: RelativeRange[] },
): string => {
  if (value.preset !== CUSTOM_RANGE) {
    return findRange(value.preset, options?.ranges)?.label ?? "Any time";
  }
  const from = toInstant(value.from);
  const to = toInstant(value.to);
  if (from && to) return `${formatDateTime(from)} \u2192 ${formatDateTime(to)}`;
  if (from) return `After ${formatDateTime(from)}`;
  if (to) return `Before ${formatDateTime(to)}`;
  return "Custom range";
};

export const browserTimeZone = (): string => {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZoneName: "short",
  }).formatToParts(new Date());
  return parts.find((part) => part.type === "timeZoneName")?.value ?? "";
};

type TimeRangePickerProps = {
  value: TimeRangeValue;
  onChange: (next: TimeRangeValue) => void;
  errorId: string;
  minWidth?: number;
  ranges?: RelativeRange[];
  allowAnyTime?: boolean;
};

const TimeRangePicker: React.FC<TimeRangePickerProps> = ({
  value,
  onChange,
  errorId,
  minWidth = 230,
  ranges = RELATIVE_RANGES,
  allowAnyTime = true,
}) => {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const [draft, setDraft] = useState<TimeRangeValue | null>(null);
  const edited = draft ?? value;
  const errors = timeRangeFieldErrors(edited, !allowAnyTime);
  const sharedError = Boolean(errors.from) && errors.from === errors.to;
  const label = timeRangeLabel(value, { ranges });
  const presetBounds = useMemo(() => {
    const resolved = resolveTimeRangeFilter(timeRangeFilter(edited), {
      ranges,
    });
    if (resolved.from && !resolved.to) {
      return { ...resolved, to: new Date().toISOString() };
    }
    return resolved;
  }, [edited, ranges]);
  const isCustom = edited.preset === CUSTOM_RANGE;
  const dayGranularity =
    ranges.length > 0 && ranges.every((range) => range.anchor === "day");
  const formatBound = dayGranularity ? toDateInputValue : toInputValue;
  const formatTo = dayGranularity ? toLastDateInputValue : toInputValue;
  const bounds = {
    from: formatBound(isCustom ? edited.from : presetBounds.from),
    to: formatTo(isCustom ? edited.to : presetBounds.to),
  };

  const editBound = (field: "from" | "to", next: string) => {
    const toValue = field === "to" ? next : bounds.to;
    const candidate: TimeRangeValue = {
      preset: CUSTOM_RANGE,
      from: toInstant(field === "from" ? next : bounds.from),
      to: dayGranularity ? fromLastDateInputValue(toValue) : toInstant(toValue),
    };
    const candidateErrors = timeRangeFieldErrors(candidate, !allowAnyTime);
    if (candidateErrors.from || candidateErrors.to) {
      setDraft(candidate);
      return;
    }
    setDraft(null);
    onChange(candidate);
  };

  const applyPreset = (preset: string) => {
    setDraft(null);
    onChange({ ...value, preset });
    setAnchor(null);
  };

  return (
    <>
      <Tooltip title={anchor || value.preset !== CUSTOM_RANGE ? "" : label}>
        <Button
          variant="outlined"
          color="inherit"
          onClick={(event) => setAnchor(event.currentTarget)}
          startIcon={<AccessTimeIcon fontSize="small" />}
          endIcon={<ArrowDropDownIcon />}
          aria-haspopup="true"
          aria-expanded={Boolean(anchor)}
          aria-label={`Time range: ${label}`}
          sx={{
            height: 40,
            minWidth,
            maxWidth: 300,
            justifyContent: "space-between",
            textTransform: "none",
            color: "text.primary",
            borderColor: errors.from || errors.to ? "error.main" : "divider",
          }}
        >
          <Box
            component="span"
            sx={{
              flexGrow: 1,
              textAlign: "left",
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
          >
            {label}
          </Box>
        </Button>
      </Tooltip>
      <Popover
        open={Boolean(anchor)}
        anchorEl={anchor}
        onClose={() => {
          setAnchor(null);
          setDraft(null);
        }}
        anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
        slotProps={{ paper: { sx: { mt: 1 } } }}
      >
        <Box>
          <Box
            sx={{
              display: "flex",
              flexDirection: { xs: "column", sm: "row" },
            }}
          >
            <Box
              sx={{
                p: 2,
                width: 300,
                display: "flex",
                flexDirection: "column",
                gap: 2,
              }}
            >
              <Typography variant="subtitle2">Custom range</Typography>
              <TextField
                label="From"
                type={dayGranularity ? "date" : "datetime-local"}
                value={bounds.from}
                onChange={(event) => editBound("from", event.target.value)}
                error={!!errors.from}
                helperText={sharedError ? undefined : errors.from}
                size="small"
                slotProps={{
                  inputLabel: { shrink: true },
                  formHelperText: { id: `${errorId}-from`, role: "alert" },
                  htmlInput: {
                    "aria-describedby": errors.from
                      ? sharedError
                        ? errorId
                        : `${errorId}-from`
                      : undefined,
                  },
                }}
              />
              <TextField
                label="To"
                type={dayGranularity ? "date" : "datetime-local"}
                value={bounds.to}
                onChange={(event) => editBound("to", event.target.value)}
                error={!!errors.to}
                helperText={errors.to}
                size="small"
                slotProps={{
                  inputLabel: { shrink: true },
                  formHelperText: { id: errorId, role: "alert" },
                  htmlInput: {
                    min: (isCustom && bounds.from) || undefined,
                    "aria-describedby": errors.to ? errorId : undefined,
                  },
                }}
              />
            </Box>
            <Box
              sx={{
                borderColor: "divider",
                borderLeft: { sm: 1 },
                borderTop: { xs: 1, sm: 0 },
                minWidth: 220,
              }}
            >
              <MenuList>
                {allowAnyTime && (
                  <MenuItem
                    selected={value.preset === ANY_RANGE}
                    onClick={() => applyPreset(ANY_RANGE)}
                  >
                    Any time
                  </MenuItem>
                )}
                {ranges.map((range) => (
                  <MenuItem
                    key={range.value}
                    selected={value.preset === range.value}
                    onClick={() => applyPreset(range.value)}
                  >
                    {range.label}
                  </MenuItem>
                ))}
              </MenuList>
            </Box>
          </Box>
          <Box sx={{ borderTop: 1, borderColor: "divider", px: 2, py: 1 }}>
            <Typography variant="caption" color="text.secondary">
              Browser time: {browserTimeZone()}
            </Typography>
          </Box>
        </Box>
      </Popover>
    </>
  );
};

export default TimeRangePicker;
