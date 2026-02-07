import React from "react";
import { Box, Typography, Button } from "@mui/material";

interface EmptyStateProps {
  primaryText: string;
  secondaryText: string;
  extraContent?: React.ReactNode;
  button: boolean;
  buttonText: string;
  onCreate: () => void;
}

const EmptyState: React.FC<EmptyStateProps> = ({
  primaryText,
  secondaryText,
  extraContent,
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

      {extraContent && (
        <Box sx={{ mt: 1, width: "100%" }}>
          {typeof extraContent === "string" ? (
            <Typography variant="body1" color="text.secondary">
              {extraContent}
            </Typography>
          ) : (
            extraContent
          )}
        </Box>
      )}

      {button && (
        <Button
          variant="contained"
          color="success"
          onClick={onCreate}
          sx={{ mt: 2 }}
        >
          {buttonText}
        </Button>
      )}
    </Box>
  );
};

export default EmptyState;
