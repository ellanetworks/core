import React, { useState, useEffect, useCallback } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
  Alert,
  Collapse,
} from "@mui/material";
import * as yup from "yup";
import { updateMyUserPassword } from "@/queries/users";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

const schema = yup.object().shape({
  currentPassword: yup
    .string()
    .min(1, "Current password is required")
    .required("Current password is required"),
  password: yup
    .string()
    .min(1, "New password is required")
    .required("New password is required"),
});

interface EditMyUserPasswordModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

interface FormValues {
  currentPassword: string;
  password: string;
}

const EditMyUserPasswordModal: React.FC<EditMyUserPasswordModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState<FormValues>({
    currentPassword: "",
    password: "",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

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

  useEffect(() => {
    if (open) {
      setFormValues({ currentPassword: "", password: "" });
      setErrors({});
      setTouched({});
      setIsValid(false);
    }
  }, [open]);

  const validateField = async (field: string, value: string) => {
    try {
      await schema.validateAt(field, { [field]: value });
      setErrors((prev) => {
        const next = { ...prev };
        delete next[field];
        return next;
      });
    } catch (err: unknown) {
      if (err instanceof yup.ValidationError) {
        setErrors((prev) => ({ ...prev, [field]: err.message }));
      }
    }
  };

  const handleChange = (field: keyof FormValues, value: string) => {
    setFormValues((prev) => ({ ...prev, [field]: value }));
    validateField(field, value);
  };

  const handleBlur = (field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateMyUserPassword(
        accessToken,
        formValues.currentPassword,
        formValues.password,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      let errorMessage = "Unknown error occurred.";
      if (error instanceof Error) {
        errorMessage = error.message;
      }
      setAlert({
        message: `Failed to update password: ${errorMessage.replace(/^\d{3}: [A-Za-z ]+\.\s*/, "")}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-my-user-password-modal-title"
      aria-describedby="edit-my-user-password-modal-description"
    >
      <DialogTitle id="edit-my-user-password-modal-title">
        Change Password
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
        <TextField
          fullWidth
          required
          label="Current Password"
          type="password"
          value={formValues.currentPassword}
          onChange={(e) => handleChange("currentPassword", e.target.value)}
          onBlur={() => handleBlur("currentPassword")}
          error={!!errors.currentPassword && touched.currentPassword}
          helperText={touched.currentPassword ? errors.currentPassword : ""}
          margin="normal"
          autoFocus
          autoComplete="current-password"
        />
        <TextField
          fullWidth
          required
          label="New Password"
          type="password"
          value={formValues.password}
          onChange={(e) => handleChange("password", e.target.value)}
          onBlur={() => handleBlur("password")}
          error={!!errors.password && touched.password}
          helperText={touched.password ? errors.password : ""}
          margin="normal"
          autoComplete="new-password"
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

export default EditMyUserPasswordModal;
