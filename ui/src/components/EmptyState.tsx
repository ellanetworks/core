// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Box, Typography } from "@mui/material";

interface EmptyStateProps {
  primaryText: string;
  secondaryText: string;
}

const EmptyState: React.FC<EmptyStateProps> = ({
  primaryText,
  secondaryText,
}) => {
  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "flex-start",
        justifyContent: "flex-start",
        margin: "0 auto",
        padding: 2,
        width: "100%",
        maxWidth: 640,
        marginTop: 4,
      }}
    >
      <Typography variant="h6" component="p" gutterBottom align="left">
        {primaryText}
      </Typography>

      <Typography variant="body1" gutterBottom align="left">
        {secondaryText}
      </Typography>
    </Box>
  );
};

export default EmptyState;
