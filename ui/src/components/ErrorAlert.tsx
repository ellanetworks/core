// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import {
  Alert,
  AlertTitle,
  Box,
  Button,
  Collapse,
  IconButton,
  Link,
  Typography,
} from "@mui/material";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import { ApiError } from "@/queries/utils";

interface ErrorAlertProps {
  resource: string;
  error: unknown;
  onRetry?: () => void;
  retrying?: boolean;
}

const primaryMessage = (error: unknown): string => {
  if (error instanceof ApiError) return error.message;
  if (error instanceof Error) return error.message;
  return "An unexpected error occurred.";
};

const technicalDetail = (error: unknown): string | undefined => {
  if (error instanceof ApiError) return error.detail;
  if (error instanceof Error) return error.stack;
  return undefined;
};

const ErrorAlert: React.FC<ErrorAlertProps> = ({
  resource,
  error,
  onRetry,
  retrying = false,
}) => {
  const [showDetail, setShowDetail] = useState(false);
  const detail = technicalDetail(error);

  const copyDetail = () => {
    if (detail) void navigator.clipboard.writeText(detail);
  };

  return (
    <Alert
      severity="error"
      sx={{ mt: 2 }}
      action={
        onRetry && (
          <Button
            color="inherit"
            size="small"
            onClick={onRetry}
            disabled={retrying}
          >
            {retrying ? "Retrying…" : "Retry"}
          </Button>
        )
      }
    >
      <AlertTitle>Failed to load {resource}</AlertTitle>
      <Typography variant="body2">{primaryMessage(error)}</Typography>

      {detail && (
        <Box sx={{ mt: 1 }}>
          <Link
            component="button"
            type="button"
            variant="body2"
            underline="hover"
            onClick={() => setShowDetail((shown) => !shown)}
          >
            {showDetail ? "Hide details" : "Show details"}
          </Link>
          <Collapse in={showDetail}>
            <Box
              sx={{
                mt: 1,
                display: "flex",
                alignItems: "flex-start",
                gap: 1,
              }}
            >
              <Typography
                variant="body2"
                component="pre"
                sx={{
                  m: 0,
                  whiteSpace: "pre-wrap",
                  overflowX: "auto",
                  flex: 1,
                }}
              >
                {detail}
              </Typography>
              <IconButton
                size="small"
                onClick={copyDetail}
                aria-label={`Copy ${resource} error details`}
              >
                <ContentCopyIcon fontSize="inherit" />
              </IconButton>
            </Box>
          </Collapse>
        </Box>
      )}
    </Alert>
  );
};

export default ErrorAlert;
