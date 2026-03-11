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
import { ValidationError } from "yup";
import { updateOperatorCode } from "@/queries/operator";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditOperatorCodeModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object().shape({
  operatorCode: yup
    .string()
    .required("Operator Code is required.")
    .matches(
      /^[0-9A-Fa-f]{32}$/,
      "Operator Code must be a 32-character hexadecimal string.",
    ),
});

interface FormValues {
  operatorCode: string;
}

const EditOperatorCodeModal: React.FC<EditOperatorCodeModalProps> = ({
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
    operatorCode: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({ operatorCode: "" });
      setErrors({});
      setTouched({});
    }
  }, [open]);

  const handleChange = (field: keyof FormValues, value: string) => {
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
      const fieldSchema = yup.reach(schema, field) as yup.Schema<unknown>;
      await fieldSchema.validate(value);
      setErrors((prev) => ({ ...prev, [field]: "" }));
    } catch (err) {
      if (err instanceof ValidationError) {
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
      await updateOperatorCode(accessToken, formValues.operatorCode);
      onClose();
      onSuccess();
    } catch (error) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({ message: `Failed to update operator code: ${errorMessage}` });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-operator-code-modal-title"
      aria-describedby="edit-operator-code-modal-description"
    >
      <DialogTitle>Edit Operator Code</DialogTitle>
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
        <DialogContentText id="edit-operator-code-modal-description">
          The Operator Code (OP) is a secret identifier used to authenticate the
          operator and provision SIM cards. Keep this code secure as it
          can&apos;t be retrieved once set.
        </DialogContentText>
        <Alert severity="warning" sx={{ mt: 2, mb: 2 }}>
          This operation cannot be undone. The operator code cannot be retrieved
          once set.
        </Alert>
        <TextField
          fullWidth
          label="Operator Code"
          value={formValues.operatorCode}
          onChange={(e) => handleChange("operatorCode", e.target.value)}
          onBlur={() => handleBlur("operatorCode")}
          error={touched.operatorCode && !!errors.operatorCode}
          helperText={touched.operatorCode ? errors.operatorCode : ""}
          margin="normal"
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

export default EditOperatorCodeModal;
