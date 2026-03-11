import React, { useCallback, useState, useEffect } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  DialogContentText,
  TextField,
  Button,
  Alert,
  Collapse,
} from "@mui/material";
import * as yup from "yup";
import {
  updateUsageRetentionPolicy,
  type UsageRetentionPolicy,
} from "@/queries/usage";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

const schema = yup.object().shape({
  days: yup
    .number()
    .typeError("Days must be a number")
    .integer("Days must be a whole number")
    .min(1, "Must retain data for at least 1 day")
    .max(3650, "Retention cannot exceed 3650 days (10 years)")
    .required("Days is required"),
});

interface EditUsageRetentionPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: UsageRetentionPolicy;
}

const EditUsageRetentionPolicyModal: React.FC<
  EditUsageRetentionPolicyModalProps
> = ({ open, onClose, onSuccess, initialData }) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState(initialData);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [isValid, setIsValid] = useState(true);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({
        days: initialData.days,
      });
      setErrors({});
    }
  }, [open, initialData]);

  const handleChange = (field: string, value: number) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));
    validateField(field, value);
  };

  const validateField = async (field: string, value: number) => {
    try {
      const fieldSchema = yup.reach(schema, field) as yup.Schema<unknown>;
      await fieldSchema.validate(value);
      setErrors((prev) => ({ ...prev, [field]: "" }));
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
      await updateUsageRetentionPolicy(accessToken, formValues.days);
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update usage retention policy: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-usage-retention-policy-modal-title"
      aria-describedby="edit-usage-retention-policy-modal-description"
    >
      <DialogTitle id="edit-usage-retention-policy-modal-title">
        Edit Usage Retention Policy
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
        <DialogContentText>
          Set the number of days to retain usage data. After this period, data
          will be automatically deleted.
        </DialogContentText>
        <TextField
          fullWidth
          type="number"
          label="Days"
          value={formValues.days}
          onChange={(e) => handleChange("days", Number(e.target.value))}
          error={!!errors.days}
          helperText={errors.days || "Enter a value between 1 and 3650."}
          margin="normal"
          autoFocus
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

export default EditUsageRetentionPolicyModal;
