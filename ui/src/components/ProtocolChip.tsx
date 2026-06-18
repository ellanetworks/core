// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Chip, Tooltip } from "@mui/material";

// The control-plane protocol maps to a radio generation: NGAP is 5G, S1AP is 4G.
const PROTOCOL_GENERATION: Record<string, string> = {
  NGAP: "5G",
  S1AP: "4G",
};

/**
 * ProtocolChip renders a neutral control-plane protocol tag ("NGAP" / "S1AP").
 * A protocol isn't a success/error state, so the chip is colour-neutral, and the
 * radio generation it belongs to is shown as a tooltip. Used in the network
 * events table, drawer header, and drawer metadata so the styling stays
 * consistent.
 */
const ProtocolChip: React.FC<{ protocol?: string | null }> = ({ protocol }) => {
  if (!protocol) return null;

  const generation = PROTOCOL_GENERATION[protocol];
  const chip = (
    <Chip size="small" label={protocol} color="default" variant="filled" />
  );

  if (!generation) return chip;

  return <Tooltip title={generation}>{chip}</Tooltip>;
};

export default ProtocolChip;
