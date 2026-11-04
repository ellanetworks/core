// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Chip, Tooltip } from "@mui/material";
import type { N32State, RoamingPartnerN32 } from "@/queries/roaming";

const stateLabels: Record<
  N32State,
  { label: string; color: "success" | "warning" | "error" | "default" }
> = {
  established: { label: "Established", color: "success" },
  establishing: { label: "Establishing", color: "warning" },
  failed: { label: "Failed", color: "error" },
  terminated: { label: "Terminated", color: "error" },
  idle: { label: "Idle", color: "default" },
  unknown: { label: "Unknown", color: "default" },
};

const N32StatusChip: React.FC<{ n32: RoamingPartnerN32 }> = ({ n32 }) => {
  const { label, color } = stateLabels[n32.state] ?? {
    label: n32.state,
    color: "default",
  };

  return (
    <Tooltip title={n32.lastError ?? ""} arrow>
      <Chip label={label} color={color} size="small" variant="outlined" />
    </Tooltip>
  );
};

export default N32StatusChip;
