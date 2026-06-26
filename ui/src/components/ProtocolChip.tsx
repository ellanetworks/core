// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Chip } from "@mui/material";

// The control-plane protocol maps to a radio generation: NGAP is 5G, S1AP is 4G.
const PROTOCOL_GENERATION: Record<string, string> = {
  NGAP: "5G",
  S1AP: "4G",
};

/**
 * ProtocolChip renders a neutral control-plane protocol tag with its radio
 * generation ("NGAP (5G)" / "S1AP (4G)"). A protocol isn't a success/error
 * state, so the chip is colour-neutral. Used in the network events table,
 * drawer header, and drawer metadata so the styling stays consistent.
 */
const ProtocolChip: React.FC<{ protocol?: string | null }> = ({ protocol }) => {
  if (!protocol) return null;

  const generation = PROTOCOL_GENERATION[protocol];
  const label = generation ? `${protocol} (${generation})` : protocol;

  return <Chip size="small" label={label} color="default" variant="filled" />;
};

export default ProtocolChip;
