// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Chip } from "@mui/material";
import { useTheme } from "@mui/material/styles";
import { formatProtocol } from "@/utils/formatters";

/** A network rule with protocol 0 matches any protocol. */
const IPProtocolChip: React.FC<{ protocol: number; color?: string }> = ({
  protocol,
  color,
}) => {
  const theme = useTheme();

  return (
    <Chip
      size="small"
      label={protocol === 0 ? "any" : formatProtocol(protocol)}
      sx={{
        backgroundColor:
          color ??
          theme.palette.chart.protocols[protocol] ??
          theme.palette.grey[600],
        color: "#fff",
        fontWeight: 600,
        fontSize: "0.75rem",
        height: 22,
      }}
    />
  );
};

export default IPProtocolChip;
