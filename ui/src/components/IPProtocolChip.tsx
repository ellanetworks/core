// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Chip } from "@mui/material";
import { formatProtocol, PROTOCOL_CHIP_COLORS } from "@/utils/formatters";

/** A network rule with protocol 0 matches any protocol. */
const IPProtocolChip: React.FC<{ protocol: number; color?: string }> = ({
  protocol,
  color,
}) => (
  <Chip
    size="small"
    label={protocol === 0 ? "any" : formatProtocol(protocol)}
    sx={{
      backgroundColor: color ?? PROTOCOL_CHIP_COLORS[protocol] ?? "grey.600",
      color: "#fff",
      fontWeight: 600,
      fontSize: "0.75rem",
      height: 22,
    }}
  />
);

export default IPProtocolChip;
