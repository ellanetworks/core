"use client";

import React from "react";
import { Box, Typography, Button } from "@mui/material";

interface EmptyStateProps {
  primaryText: string;
  secondaryText: string;
  button: boolean;
  buttonText: string;
  onCreate: () => void;
}

const EmptyState: React.FC<EmptyStateProps> = ({
  primaryText,
  secondaryText,
  button,
  buttonText,
  onCreate,
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
        width: "50%",
        marginTop: 4,
      }}
    >
      <Typography variant="h4" gutterBottom align="left">
        {primaryText}
      </Typography>
      <Typography variant="h6" gutterBottom align="left">
        {secondaryText}
      </Typography>
      {button && (
        <Button variant="contained" color="success" onClick={onCreate}>
          {buttonText}
        </Button>
      )}
    </Box>
  );
};

export default EmptyState;
