// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { Alert, AlertTitle, Box, Button, Collapse, Link } from "@mui/material";

interface ErrorBoundaryProps {
  children: React.ReactNode;
}

interface ErrorBoundaryState {
  error: Error | null;
  showDetail: boolean;
}

/**
 * A render exception anywhere below this point would otherwise unmount the whole
 * tree and leave a blank page, so it is caught here and reported in place.
 *
 * Must be a class: React exposes no hook equivalent of componentDidCatch.
 */
class ErrorBoundary extends React.Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  state: ErrorBoundaryState = { error: null, showDetail: false };

  static getDerivedStateFromError(error: Error): Partial<ErrorBoundaryState> {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error("Unhandled UI error:", error, info.componentStack);
  }

  render() {
    const { error, showDetail } = this.state;
    if (!error) return this.props.children;

    const detail = [error.stack, error.message].find(Boolean);

    return (
      <Box sx={{ p: 4, maxWidth: 720, mx: "auto" }}>
        <Alert
          severity="error"
          action={
            <Button
              color="inherit"
              size="small"
              onClick={() => window.location.reload()}
            >
              Reload
            </Button>
          }
        >
          <AlertTitle>Something went wrong in the interface</AlertTitle>
          This page stopped responding because of an error in Ella Core&apos;s
          web interface. Your network is unaffected. Reloading usually clears
          it.
          {detail && (
            <Box sx={{ mt: 1 }}>
              <Link
                component="button"
                type="button"
                variant="body2"
                underline="hover"
                onClick={() =>
                  this.setState((s) => ({ showDetail: !s.showDetail }))
                }
              >
                {showDetail ? "Hide details" : "Show details"}
              </Link>
              <Collapse in={showDetail}>
                <Box
                  component="pre"
                  sx={{
                    mt: 1,
                    fontSize: "0.75rem",
                    whiteSpace: "pre-wrap",
                    overflowX: "auto",
                    maxHeight: 240,
                  }}
                >
                  {detail}
                </Box>
              </Collapse>
            </Box>
          )}
        </Alert>
      </Box>
    );
  }
}

export default ErrorBoundary;
