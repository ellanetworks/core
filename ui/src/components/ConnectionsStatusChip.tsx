// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Chip, Tooltip } from "@mui/material";
import DiameterPeerStatusChip, {
  diameterPeerStateLabel,
} from "@/components/DiameterPeerStatusChip";
import type { DiameterPeerStatus } from "@/queries/integrations";

export interface Connection {
  label: string;
  status?: DiameterPeerStatus;
}

const describe = ({ label, status }: Connection) => {
  const state = diameterPeerStateLabel(status?.state ?? "connecting");
  return status?.error && status.state !== "open"
    ? `${label}: ${state} (${status.error})`
    : `${label}: ${state}`;
};

const ConnectionsStatusChip: React.FC<{
  connections: Connection[];
  emptyLabel?: string;
}> = ({ connections, emptyLabel = "Not connected" }) => {
  if (connections.length === 0) {
    return <Chip size="small" variant="outlined" label={emptyLabel} />;
  }

  if (connections.length === 1) {
    return <DiameterPeerStatusChip status={connections[0].status} />;
  }

  const open = connections.filter((c) => c.status?.state === "open").length;
  const color =
    open === connections.length ? "success" : open === 0 ? "error" : "warning";

  return (
    <Tooltip
      arrow
      title={connections.map(describe).join("\n")}
      slotProps={{ tooltip: { sx: { whiteSpace: "pre-line" } } }}
    >
      <Chip
        size="small"
        variant="outlined"
        color={color}
        label={`${open} of ${connections.length} connected`}
      />
    </Tooltip>
  );
};

export default ConnectionsStatusChip;
