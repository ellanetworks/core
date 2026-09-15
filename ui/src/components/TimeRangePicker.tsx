// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import {
  Alert,
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
import { formatDate, formatDateTime } from "@/utils/formatters";
import { localDateString } from "@/utils/dates";

export const CUSTOM_RANGE = "custom";

export type TimeRangeValue = {
  preset: string;
  from: string;
  to: string;
};

export const EMPTY_TIME_RANGE: TimeRangeValue = {
  preset: "",
  from: "",
  to: "",
};

export type TimeRangeFilter = {
  relative?: string;
  from?: string;
  to?: string;
};

export type TimeRangeGranularity = "datetime" | "date";

export type RelativeRange = {
  value: string;
  label: string;
  ms: number;
  endMs?: number;
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

export const DAILY_RANGES: RelativeRange[] = [
  { value: "today", label: "Today", ms: 0, endMs: 0 },
  { value: "yesterday", label: "Yesterday", ms: DAY_MS, endMs: DAY_MS },
  { value: "7d", label: "Last 7 days", ms: 6 * DAY_MS, endMs: 0 },
  { value: "30d", label: "Last 30 days", ms: 29 * DAY_MS, endMs: 0 },
  { value: "90d", label: "Last 90 days", ms: 89 * DAY_MS, endMs: 0 },
];

const toIsoInstant = (value: string): string => {
  if (!value) return "";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime()) ? "" : parsed.toISOString();
};

export const timeRangeError = (value: TimeRangeValue): string => {
  if (value.preset !== CUSTOM_RANGE) return "";
  const fromIso = toIsoInstant(value.from);
  const toIso = toIsoInstant(value.to);
  if ((value.from && !fromIso) || (value.to && !toIso)) {
    return "Enter a valid date and time.";
  }
  if (fromIso && toIso && fromIso > toIso) {
    return "The To timestamp must be on or after the From timestamp.";
  }
  return "";
};

export const timeRangeFilter = (
  value: TimeRangeValue,
  granularity: TimeRangeGranularity = "datetime",
): TimeRangeFilter => {
  if (value.preset === CUSTOM_RANGE) {
    const filter: TimeRangeFilter = {};
    const from = granularity === "date" ? value.from : toIsoInstant(value.from);
    const to = granularity === "date" ? value.to : toIsoInstant(value.to);
    if (from) filter.from = from;
    if (to) filter.to = to;
    return filter;
  }
  return value.preset ? { relative: value.preset } : {};
};

export const toInputValue = (
  stampValue: string | undefined,
  granularity: TimeRangeGranularity,
): string => {
  if (!stampValue) return "";
  if (granularity === "date") return stampValue;
  const parsed = new Date(stampValue);
  if (Number.isNaN(parsed.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${parsed.getFullYear()}-${pad(parsed.getMonth() + 1)}-${pad(
    parsed.getDate(),
  )}T${pad(parsed.getHours())}:${pad(parsed.getMinutes())}`;
};

const stamp = (offsetMs: number, granularity: TimeRangeGranularity): string => {
  const at = new Date(Date.now() - offsetMs);
  return granularity === "date" ? localDateString(at) : at.toISOString();
};

export const resolveTimeRangeFilter = (
  filter: TimeRangeFilter,
  options?: {
    ranges?: RelativeRange[];
    granularity?: TimeRangeGranularity;
  },
): { from?: string; to?: string } => {
  const granularity = options?.granularity ?? "datetime";
  if (!filter.relative) {
    const resolved: { from?: string; to?: string } = {};
    if (filter.from) resolved.from = filter.from;
    if (filter.to) resolved.to = filter.to;
    return resolved;
  }
  const range = (options?.ranges ?? RELATIVE_RANGES).find(
    (candidate) => candidate.value === filter.relative,
  );
  if (!range) return {};
  const resolved: { from?: string; to?: string } = {
    from: stamp(range.ms, granularity),
  };
  if (range.endMs !== undefined) resolved.to = stamp(range.endMs, granularity);
  return resolved;
};

const DATE_ONLY = /^(\d{4})-(\d{2})-(\d{2})$/;

const formatDateOnly = (value: string): string => {
  const match = DATE_ONLY.exec(value);
  if (!match) return formatDate(value);
  const [, year, month, day] = match;
  return new Date(
    Number(year),
    Number(month) - 1,
    Number(day),
  ).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
};

export const timeRangeLabel = (
  value: TimeRangeValue,
  options?: {
    ranges?: RelativeRange[];
    granularity?: TimeRangeGranularity;
  },
): string => {
  const ranges = options?.ranges ?? RELATIVE_RANGES;
  if (value.preset !== CUSTOM_RANGE) {
    return (
      ranges.find((range) => range.value === value.preset)?.label ?? "Any time"
    );
  }
  const isDateOnly = options?.granularity === "date";
  const format = isDateOnly ? formatDateOnly : formatDateTime;
  const from = isDateOnly ? value.from : toIsoInstant(value.from);
  const to = isDateOnly ? value.to : toIsoInstant(value.to);
  if (from && to) return `${format(from)} \u2192 ${format(to)}`;
  if (from) return `After ${format(from)}`;
  if (to) return `Before ${format(to)}`;
  return "Custom range";
};

type TimeRangePickerProps = {
  value: TimeRangeValue;
  onChange: (next: TimeRangeValue) => void;
  errorId: string;
  minWidth?: number;
  ranges?: RelativeRange[];
  granularity?: TimeRangeGranularity;
  allowAnyTime?: boolean;
  error?: string;
};

const TimeRangePicker: React.FC<TimeRangePickerProps> = ({
  value,
  onChange,
  errorId,
  minWidth = 230,
  ranges = RELATIVE_RANGES,
  granularity = "datetime",
  allowAnyTime = true,
  error: errorOverride,
}) => {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const error = errorOverride ?? timeRangeError(value);
  const label = timeRangeLabel(value, { ranges, granularity });
  const inputType = granularity === "date" ? "date" : "datetime-local";
  const presetBounds = resolveTimeRangeFilter(
    timeRangeFilter(value, granularity),
    { ranges, granularity },
  );
  const bounds =
    value.preset === CUSTOM_RANGE
      ? { from: value.from, to: value.to }
      : {
          from: toInputValue(presetBounds.from, granularity),
          to: toInputValue(presetBounds.to, granularity),
        };

  const applyPreset = (preset: string) => {
    onChange({ ...value, preset });
    setAnchor(null);
  };

  return (
    <>
      <Tooltip
        title={
          anchor ? "" : error || (value.preset === CUSTOM_RANGE ? label : "")
        }
      >
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
            borderColor: error ? "error.main" : "divider",
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
        onClose={() => setAnchor(null)}
        anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
        slotProps={{ paper: { sx: { mt: 1 } } }}
      >
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
              type={inputType}
              value={bounds.from}
              onChange={(event) =>
                onChange({
                  ...bounds,
                  preset: CUSTOM_RANGE,
                  from: event.target.value,
                })
              }
              error={!!error}
              size="small"
              slotProps={{
                inputLabel: { shrink: true },
                htmlInput: {
                  "aria-describedby": error ? errorId : undefined,
                },
              }}
            />
            <TextField
              label="To"
              type={inputType}
              value={bounds.to}
              onChange={(event) =>
                onChange({
                  ...bounds,
                  preset: CUSTOM_RANGE,
                  to: event.target.value,
                })
              }
              error={!!error}
              size="small"
              slotProps={{
                inputLabel: { shrink: true },
                htmlInput: {
                  min: bounds.from || undefined,
                  "aria-describedby": error ? errorId : undefined,
                },
              }}
            />
            {error && (
              <Alert severity="error" id={errorId}>
                {error}
              </Alert>
            )}
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
                  selected={value.preset === ""}
                  onClick={() => applyPreset("")}
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
      </Popover>
    </>
  );
};

export default TimeRangePicker;
