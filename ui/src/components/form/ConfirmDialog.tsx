// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useId, useState } from "react";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Collapse,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Link,
} from "@mui/material";
import type { ButtonProps, DialogProps } from "@mui/material";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import ExpandLessIcon from "@mui/icons-material/ExpandLess";

interface ConfirmDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void>;
  title: string;
  description: React.ReactNode;
  details?: { label: string; content: React.ReactNode };
  confirmLabel?: string;
  confirmingLabel?: string;
  confirmColor?: ButtonProps["color"];
  maxWidth?: DialogProps["maxWidth"];
  fullWidth?: boolean;
}

const ConfirmDialog: React.FC<ConfirmDialogProps> = ({
  open,
  onClose,
  onConfirm,
  title,
  description,
  details,
  confirmLabel = "Confirm",
  confirmingLabel = "Confirming…",
  confirmColor = "error",
  maxWidth = "sm",
  fullWidth = false,
}) => {
  const baseId = useId();
  const titleId = `${baseId}-title`;
  const descriptionId = `${baseId}-description`;
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [showDetails, setShowDetails] = useState(false);

  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) {
      setError("");
      setShowDetails(false);
    }
  }

  const handleConfirm = async () => {
    setLoading(true);
    setError("");
    try {
      await onConfirm();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={loading ? undefined : onClose}
      aria-labelledby={titleId}
      aria-describedby={descriptionId}
      maxWidth={maxWidth}
      fullWidth={fullWidth}
    >
      <DialogTitle id={titleId}>{title}</DialogTitle>
      <DialogContent dividers>
        <Collapse in={!!error}>
          <Alert severity="error" onClose={() => setError("")} sx={{ mb: 2 }}>
            {error}
          </Alert>
        </Collapse>
        <DialogContentText id={descriptionId}>{description}</DialogContentText>

        {details && (
          <Box sx={{ mt: 1 }}>
            <Link
              component="button"
              type="button"
              variant="body2"
              underline="hover"
              onClick={() => setShowDetails((value) => !value)}
              sx={{ display: "inline-flex", alignItems: "center" }}
            >
              {showDetails ? (
                <ExpandLessIcon fontSize="small" />
              ) : (
                <ExpandMoreIcon fontSize="small" />
              )}
              {details.label}
            </Link>
            <Collapse in={showDetails}>{details.content}</Collapse>
          </Box>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={loading} sx={{ marginRight: 2 }}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color={confirmColor}
          onClick={handleConfirm}
          disabled={loading}
          startIcon={
            loading ? <CircularProgress size={16} color="inherit" /> : undefined
          }
        >
          {loading ? confirmingLabel : confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ConfirmDialog;
