// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Chip } from "@mui/material";
import type { RadioStatus } from "@/queries/radios";

const RadioStatusChip: React.FC<{ status: RadioStatus }> = ({ status }) => (
  <Chip
    size="small"
    label={status === "online" ? "Online" : "Offline"}
    color={status === "online" ? "success" : "default"}
    variant="filled"
  />
);

export default RadioStatusChip;
