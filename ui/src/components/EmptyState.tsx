// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Box, Typography, Button } from "@mui/material";

interface EmptyStateProps {
  primaryText: string;
  secondaryText: string;
  extraContent?: React.ReactNode;
  button?: boolean;
  buttonText?: string;
  onCreate?: () => void;
  readOnlyHint?: string;
}

const EmptyState: React.FC<EmptyStateProps> = ({
  primaryText,
  secondaryText,
  extraContent,
  button = false,
  buttonText,
  onCreate,
  readOnlyHint,
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

      {extraContent && (
        <Box sx={{ mt: 1, width: "100%" }}>
          {typeof extraContent === "string" ? (
            <Typography variant="body1" color="textSecondary">
              {extraContent}
            </Typography>
          ) : (
            extraContent
          )}
        </Box>
      )}

      {button && buttonText && onCreate && (
        <Button
          variant="contained"
          color="success"
          onClick={onCreate}
          sx={{ mt: 2 }}
        >
          {buttonText}
        </Button>
      )}

      {!button && readOnlyHint && (
        <Typography variant="body2" color="textSecondary" sx={{ mt: 2 }}>
          {readOnlyHint}
        </Typography>
      )}
    </Box>
  );
};

export default EmptyState;
