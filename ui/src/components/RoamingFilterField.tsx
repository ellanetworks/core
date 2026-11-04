// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { MenuItem, TextField } from "@mui/material";

export const roamingParam = (value: string): boolean | undefined =>
  value === "true" ? true : value === "false" ? false : undefined;

interface RoamingFilterFieldProps {
  value: string;
  onChange: (value: string) => void;
}

const RoamingFilterField: React.FC<RoamingFilterFieldProps> = ({
  value,
  onChange,
}) => (
  <TextField
    select
    label="Type"
    value={roamingParam(value) === undefined ? "" : value}
    onChange={(e) => onChange(e.target.value)}
    size="small"
    sx={{ minWidth: 160 }}
  >
    <MenuItem value="">All</MenuItem>
    <MenuItem value="false">Subscribers</MenuItem>
    <MenuItem value="true">Roamers</MenuItem>
  </TextField>
);

export default RoamingFilterField;
