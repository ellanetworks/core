import React, { useState, useEffect } from "react";
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
  updateAuditLogRetentionPolicy,
  type AuditLogRetentionPolicy,
} from "@/queries/audit_logs";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

const validationSchema = yup.object().shape({
  days: yup
    .number()
    .typeError("Days must be a number")
    .integer("Days must be a whole number")
    .min(1, "Minimum retention is 1 day")
    .max(3650, "Maximum retention is 3650 days (10 years)")
    .required("Days is required"),
});

interface EditAuditLogRetentionPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: AuditLogRetentionPolicy;
}

const EditAuditLogRetentionPolicyModal: React.FC<
  EditAuditLogRetentionPolicyModalProps
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
      setIsValid(true);
    }
  }, [open, initialData]);

  const validateField = async (field: string, value: number) => {
    try {
      await validationSchema.validateAt(field, { [field]: value });
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
    try {
      await validationSchema.validate({ ...formValues, [field]: value });
      setIsValid(true);
    } catch {
      setIsValid(false);
    }
  };

  const handleChange = (field: string, value: number) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));
    validateField(field, value);
  };

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateAuditLogRetentionPolicy(accessToken, formValues.days);
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update audit log retention policy: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-audit-log-retention-policy-modal-title"
      aria-describedby="edit-audit-log-retention-policy-modal-description"
    >
      <DialogTitle>Edit Audit Log Retention Policy</DialogTitle>
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
          Set the number of days to retain audit logs. After this period, logs
          will be automatically deleted.
        </DialogContentText>
        {formValues.days < initialData.days && formValues.days >= 1 && (
          <Alert severity="warning" sx={{ mt: 1 }}>
            Reducing retention from {initialData.days} to {formValues.days} days
            will permanently delete logs older than {formValues.days} days.
          </Alert>
        )}
        <TextField
          fullWidth
          type="number"
          label="Days"
          value={formValues.days}
          onChange={(e) => handleChange("days", Number(e.target.value))}
          error={!!errors.days}
          helperText={errors.days || "1 to 3650 days"}
          margin="normal"
          slotProps={{ input: { inputProps: { min: 1, max: 3650 } } }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={loading || !isValid}
        >
          {loading ? "Updating..." : "Update"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default EditAuditLogRetentionPolicyModal;
