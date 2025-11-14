"use client";

import React, { useState, useEffect } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  TextField,
  Button,
  Alert,
  Collapse,
  Autocomplete,
} from "@mui/material";
import * as yup from "yup";
import { updateOperatorTracking } from "@/queries/operator";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";

interface EditOperatorTrackingModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    supportedTacs: string[];
  };
}

const schema = yup
  .string()
  .matches(
    /^[0-9A-Fa-f]{6}$/,
    "Each TAC must be a 3 bytes hex string, range: 000000~FFFFFF)",
  );

const EditOperatorTrackingModal: React.FC<EditOperatorTrackingModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const router = useRouter();
  const { accessToken, authReady } = useAuth();

  if (!authReady || !accessToken) {
    router.push("/login");
  }

  const [formValues, setFormValues] = useState<{ supportedTacs: string[] }>({
    supportedTacs: [],
  });
  const [errors, setErrors] = useState<{ supportedTacs?: string }>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues(initialData);
      setErrors({});
    }
  }, [open, initialData]);

  const validateTacs = (tacs: string[]): boolean => {
    const invalidTacs = tacs.filter((tac) => !schema.isValidSync(tac));
    if (invalidTacs.length > 0) {
      setErrors({ supportedTacs: `Invalid TACs: ${invalidTacs.join(", ")}` });
      return false;
    }
    setErrors({});
    return true;
  };

  const handleTacsChange = (
    _event: React.SyntheticEvent<Element, Event>,
    value: string[],
  ) => {
    setFormValues({ supportedTacs: value });
    validateTacs(value);
  };

  const handleSubmit = async () => {
    if (!accessToken) return;
    if (!validateTacs(formValues.supportedTacs)) return;

    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateOperatorTracking(accessToken, formValues.supportedTacs);
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({ message: `Failed to update supported TACs: ${errorMessage}` });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-operator-tracking-modal-title"
      aria-describedby="edit-operator-tracking-modal-description"
    >
      <DialogTitle>Edit Operator Tracking Information</DialogTitle>
      <DialogContent dividers>
        <Collapse in={!!alert.message}>
          <Alert
            onClose={() => setAlert({ message: "" })}
            sx={{ mb: 2 }}
            severity="error"
          >
            {alert.message}
          </Alert>
        </Collapse>
        <DialogContentText
          id="edit-operator-supportedtacs-modal-description"
          sx={{ marginBottom: 3 }}
        >
          Tracking Area Codes (TACs) are used to identify a tracking area in a
          mobile network. Only radios with TACs listed here will be able to
          connect to the network.
        </DialogContentText>
        <Autocomplete
          multiple
          freeSolo
          options={[]}
          value={formValues.supportedTacs}
          onChange={handleTacsChange}
          renderInput={(params) => (
            <TextField
              {...params}
              variant="outlined"
              label="Supported TACs"
              placeholder="Enter TACs (e.g., 000001)"
              error={!!errors.supportedTacs}
              helperText={
                errors.supportedTacs ||
                "Enter each TAC as a 3 bytes hex string (e.g., 000001)"
              }
            />
          )}
          sx={{ marginBottom: 2 }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={loading}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={loading}
        >
          {loading ? "Updating..." : "Update"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default EditOperatorTrackingModal;
