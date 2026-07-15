// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { Chip } from "@mui/material";

const PROTOCOL_GENERATION: Record<string, string> = {
  NGAP: "5G",
  S1AP: "4G",
};

const ProtocolChip: React.FC<{ protocol?: string | null }> = ({ protocol }) => {
  if (!protocol) return null;

  const generation = PROTOCOL_GENERATION[protocol];
  const label = generation ? `${protocol} (${generation})` : protocol;

  return <Chip size="small" label={label} color="default" variant="filled" />;
};

export default ProtocolChip;
