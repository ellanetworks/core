// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Chip, Tooltip } from "@mui/material";
import type {
  DiameterPeerState,
  DiameterPeerStatus,
} from "@/queries/integrations";
import { formatDateTime } from "@/utils/formatters";

type ChipColor = "success" | "warning" | "error" | "default";

export const diameterPeerStateLabels: Record<
  DiameterPeerState,
  { label: string; color: ChipColor }
> = {
  open: { label: "Connected", color: "success" },
  connecting: { label: "Connecting", color: "warning" },
  reopen: { label: "Reconnecting", color: "warning" },
  suspect: { label: "Unresponsive", color: "warning" },
  closing: { label: "Disconnecting", color: "warning" },
  down: { label: "Disconnected", color: "error" },
};

export const diameterPeerStateLabel = (state: string): string =>
  diameterPeerStateLabels[state as DiameterPeerState]?.label ?? state;

const statusTooltip = (status?: DiameterPeerStatus): string => {
  if (!status) return "";
  if (status.state !== "open") return status.error ?? "";
  return [
    status.originHost
      ? `Connected to ${[status.originHost, status.realm].filter(Boolean).join(" in realm ")}`
      : "",
    `since ${formatDateTime(status.since)}.`,
  ]
    .filter(Boolean)
    .join(" ");
};

const DiameterPeerStatusChip: React.FC<{
  status?: DiameterPeerStatus;
  tooltip?: string;
}> = ({ status, tooltip }) => {
  const { label, color } = status
    ? (diameterPeerStateLabels[status.state] ?? {
        label: status.state,
        color: "default" as const,
      })
    : diameterPeerStateLabels.connecting;

  return (
    <Tooltip title={tooltip ?? statusTooltip(status)} arrow>
      <Chip label={label} color={color} size="small" variant="outlined" />
    </Tooltip>
  );
};

export default DiameterPeerStatusChip;
