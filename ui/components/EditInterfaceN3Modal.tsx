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
} from "@mui/material";
import * as yup from "yup";
import { updateN3Settings } from "@/queries/interfaces";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";

interface EditInterfaceN3ModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    externalAddress: string;
  };
}

// Strict-ish IPv4 regex (0–255 per octet)
const ipv4Regex =
  /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;

const schema = yup.object().shape({
  externalAddress: yup
    .string()
    .trim()
    .test(
      "empty-or-ipv4",
      "External address must be a valid IPv4 address (e.g., 192.168.1.10)",
      (value) => {
        // Allow empty string / undefined (means "unset → use config value")
        if (!value) return true;
        return ipv4Regex.test(value);
      },
    ),
});

const EditInterfaceN3Modal: React.FC<EditInterfaceN3ModalProps> = ({
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

  const [formValues, setFormValues] = useState<{ externalAddress: string }>({
    externalAddress: "",
  });
  const [errors, setErrors] = useState<{ externalAddress?: string }>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues(initialData);
      setErrors({});
      setAlert({ message: "" });
    }
  }, [open, initialData]);

  const handleExternalAddressChange = (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    setFormValues({ externalAddress: event.target.value });
    // Clear error as the user types
    if (errors.externalAddress) {
      setErrors((prev) => ({ ...prev, externalAddress: undefined }));
    }
  };

  const handleSubmit = async () => {
    if (!accessToken) return;

    setLoading(true);
    setAlert({ message: "" });

    // ---- Validate first ----
    try {
      await schema.validate(formValues, { abortEarly: false });
      setErrors({});
    } catch (err) {
      if (err instanceof yup.ValidationError) {
        // We only have one field here, so just surface the first message
        setErrors({ externalAddress: err.message });
        setLoading(false);
        return;
      }

      // Unknown validation error
      setAlert({ message: "Validation failed due to an unexpected error." });
      setLoading(false);
      return;
    }

    // ---- Call API ----
    try {
      await updateN3Settings(accessToken, formValues.externalAddress || "");
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update N3 external address: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-interface-n3-modal-title"
      aria-describedby="edit-interface-n3-modal-description"
    >
      <DialogTitle id="edit-interface-n3-modal-title">
        Edit N3 Interface
      </DialogTitle>
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
          id="edit-interface-n3-modal-description"
          sx={{ marginBottom: 3 }}
        >
          Configure an external IPv4 address for N3. Ella Core will advertise
          this address to radios which will use it to establish GTP tunnels. Use
          this if Ella Core is behind a proxy or NAT. If not set, Ella Core will
          use N3&apos;s address as defined in the config file.
        </DialogContentText>
        <TextField
          fullWidth
          label="External IP Address"
          value={formValues.externalAddress}
          onChange={handleExternalAddressChange}
          error={!!errors.externalAddress}
          helperText={
            errors.externalAddress ||
            "Leave empty to use N3's configured address."
          }
          margin="normal"
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

export default EditInterfaceN3Modal;
