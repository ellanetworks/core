// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Box } from "@mui/material";

const OneLine: React.FC<{ text: string }> = ({ text }) => (
  <Box
    component="span"
    title={text}
    sx={{
      display: "block",
      minWidth: 0,
      overflow: "hidden",
      textOverflow: "ellipsis",
      whiteSpace: "nowrap",
    }}
  >
    {text}
  </Box>
);

export default OneLine;
