// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Box, Chip } from "@mui/material";
import AccessChip from "@/components/AccessChip";
import type { SubscriberListStatus } from "@/queries/subscribers";

export const subscriberStatus = (
  status?: SubscriberListStatus,
): { label: string; color: "success" | "default"; rank: number } => {
  if (status?.registered) {
    return status.connection_state === "connected"
      ? { label: "Connected", color: "success", rank: 3 }
      : { label: "Idle", color: "default", rank: 2 };
  }
  if (status?.systems?.length) {
    return { label: "Registering", color: "default", rank: 1 };
  }
  return { label: "Deregistered", color: "default", rank: 0 };
};

const SubscriberStatusCell: React.FC<{ status?: SubscriberListStatus }> = ({
  status,
}) => {
  const { label, color } = subscriberStatus(status);
  const systems = status?.systems ?? [];
  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        height: "100%",
        gap: 0.5,
      }}
    >
      <Chip size="small" label={label} color={color} variant="filled" />
      {systems.map((system) => (
        <AccessChip key={system} label={system} />
      ))}
    </Box>
  );
};

export default SubscriberStatusCell;
