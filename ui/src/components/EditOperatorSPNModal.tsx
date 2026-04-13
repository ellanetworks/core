import React, { useCallback, useState, useEffect } from "react";
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
import { updateOperatorSPN } from "@/queries/operator";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditOperatorSPNModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    fullName: string;
    shortName: string;
  };
}

const MAX_SPN_LENGTH = 50;

const schema = yup.object({
  fullName: yup
    .string()
    .required("Full name is required")
    .max(
      MAX_SPN_LENGTH,
      `Full name must be at most ${MAX_SPN_LENGTH} characters`,
    ),
  shortName: yup
    .string()
    .required("Short name is required")
    .max(
      MAX_SPN_LENGTH,
      `Short name must be at most ${MAX_SPN_LENGTH} characters`,
    ),
});

const EditOperatorSPNModal: React.FC<EditOperatorSPNModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState({
    fullName: initialData.fullName,
    shortName: initialData.shortName,
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({
        fullName: initialData.fullName,
        shortName: initialData.shortName,
      });
      setErrors({});
      setTouched({});
      setAlert({ message: "" });
    }
  }, [open, initialData]);

  const handleChange = (field: "fullName" | "shortName", value: string) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));
    validateField(field, value);
  };

  const handleBlur = (field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const validateField = async (field: string, value: string) => {
    try {
      await schema.validateAt(field, { ...formValues, [field]: value });
      setErrors((prev) => {
        const next = { ...prev };
        delete next[field];
        return next;
      });
    } catch (err) {
      if (err instanceof yup.ValidationError) {
        setErrors((prev) => ({ ...prev, [field]: err.message }));
      }
    }
  };

  const validateForm = useCallback(async () => {
    try {
      await schema.validate(formValues, { abortEarly: false });
      setIsValid(true);
    } catch {
      setIsValid(false);
    }
  }, [formValues]);

  useEffect(() => {
    validateForm();
  }, [validateForm]);

  const handleSubmit = async () => {
    if (!accessToken) return;

    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateOperatorSPN(
        accessToken,
        formValues.fullName.trim(),
        formValues.shortName.trim(),
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update operator SPN: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-operator-spn-modal-title"
      aria-describedby="edit-operator-spn-modal-description"
    >
      <DialogTitle id="edit-operator-spn-modal-title">
        Edit Network Name (SPN)
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

        <DialogContentText id="edit-operator-spn-modal-description">
          The Service Provider Name (SPN) is the network name displayed on
          connected devices. Changes take effect for the next UE registration.
        </DialogContentText>

        <TextField
          fullWidth
          label="Full Name"
          value={formValues.fullName}
          onChange={(e) => handleChange("fullName", e.target.value)}
          onBlur={() => handleBlur("fullName")}
          error={touched.fullName && !!errors.fullName}
          helperText={
            (touched.fullName && errors.fullName) ||
            `The full network name shown on UE displays (max ${MAX_SPN_LENGTH} characters).`
          }
          margin="normal"
          autoFocus
          slotProps={{ htmlInput: { maxLength: MAX_SPN_LENGTH } }}
        />

        <TextField
          fullWidth
          label="Short Name"
          value={formValues.shortName}
          onChange={(e) => handleChange("shortName", e.target.value)}
          onBlur={() => handleBlur("shortName")}
          error={touched.shortName && !!errors.shortName}
          helperText={
            (touched.shortName && errors.shortName) ||
            `An abbreviated network name (max ${MAX_SPN_LENGTH} characters).`
          }
          margin="normal"
          slotProps={{ htmlInput: { maxLength: MAX_SPN_LENGTH } }}
        />
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={!isValid || loading}
        >
          {loading ? "Updating..." : "Update"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default EditOperatorSPNModal;
