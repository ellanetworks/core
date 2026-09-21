// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Box, Typography } from "@mui/material";
import { NodeId, nodeIdKey } from "@/queries/nodeId";

interface Props {
  nodeId: NodeId;
}

const NodeIdentity: React.FC<Props> = ({ nodeId }) => (
  <Box sx={{ mt: 2 }}>
    <Typography variant="caption" sx={{ color: "text.secondary" }}>
      Node identity
    </Typography>
    <Typography
      variant="body2"
      sx={{ fontFamily: "monospace", wordBreak: "break-all" }}
    >
      {nodeIdKey(nodeId)}
    </Typography>
  </Box>
);

export default NodeIdentity;
