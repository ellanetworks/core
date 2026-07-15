// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Chip } from "@mui/material";
import { formatProtocol, PROTOCOL_CHIP_COLORS } from "@/utils/formatters";

/**
 * IPProtocolChip renders an IP protocol number as a solid colour-coded tag.
 * Used for the flow table on Traffic and the network rules on a policy so that
 * a protocol reads the same wherever it appears. Control-plane protocols are a
 * separate concern; see ProtocolChip.
 *
 * A rule with protocol 0 matches any protocol, and a protocol outside
 * PROTOCOL_CHIP_COLORS has no colour assigned to it; both render neutral grey.
 *
 * `color` overrides the palette lookup, so a caller pairing chips with a chart
 * can keep a chip the same colour as its series.
 */
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
