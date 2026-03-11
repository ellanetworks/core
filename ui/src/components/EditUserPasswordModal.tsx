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
import { updateUserPassword } from "@/queries/users";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

const schema = yup.object().shape({
  password: yup
    .string()
    .min(1, "Password is required")
    .required("Password is required"),
});

interface EditUserPasswordModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    email: string;
  };
}

interface FormValues {
  email: string;
  password: string;
}

const EditUserPasswordModal: React.FC<EditUserPasswordModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();
  if (!authReady || !accessToken) navigate("/login");

  const [formValues, setFormValues] = useState<FormValues>({
    email: initialData.email,
    password: "",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const validateForm = useCallback(async () => {
    try {
      await schema.validate(
        { password: formValues.password },
        { abortEarly: false },
      );
      setIsValid(true);
    } catch {
      setIsValid(false);
    }
  }, [formValues.password]);

  useEffect(() => {
    validateForm();
  }, [validateForm]);

  useEffect(() => {
    if (open) {
      setFormValues({ email: initialData.email, password: "" });
      setErrors({});
      setTouched({});
      setIsValid(false);
    }
  }, [open, initialData]);

  const validateField = async (field: string, value: string) => {
    try {
      const fieldSchema = yup.reach(schema, field);
      await (fieldSchema as yup.StringSchema).validate(value);
      setErrors((prev) => ({ ...prev, [field]: "" }));
    } catch (err: unknown) {
      if (err instanceof yup.ValidationError) {
        setErrors((prev) => ({ ...prev, [field]: err.message }));
      }
    }
  };

  const handleChange = (field: keyof FormValues, value: string) => {
    setFormValues((prev) => ({ ...prev, [field]: value }));
    if (field === "password") validateField(field, value);
  };

  const handleBlur = (field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateUserPassword(
        accessToken,
        formValues.email,
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
      aria-labelledby="edit-user-password-modal-title"
      aria-describedby="edit-user-password-modal-description"
    >
      <DialogTitle id="edit-user-password-modal-title">
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
          label="Email"
          value={formValues.email}
          margin="normal"
          disabled
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
          autoFocus
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

export default EditUserPasswordModal;
