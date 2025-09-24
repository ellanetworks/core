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
import {
  updateAuditLogRetentionPolicy,
  type AuditLogRetentionPolicy,
} from "@/queries/audit_logs";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";

interface EditAuditLogRetentionPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: AuditLogRetentionPolicy;
}

const EditAuditLogRetentionPolicyModal: React.FC<
  EditAuditLogRetentionPolicyModalProps
> = ({ open, onClose, onSuccess, initialData }) => {
  const router = useRouter();
  const { accessToken, authReady } = useAuth();

  if (!authReady || !accessToken) {
    router.push("/login");
  }

  const [formValues, setFormValues] = useState(initialData);
  const [errors, setErrors] = useState<Record<string, string>>({});
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

  const handleChange = (field: string, value: string | number) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));
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
        <TextField
          fullWidth
          type="number"
          label="Days"
          value={formValues.days}
          onChange={(e) => handleChange("days", Number(e.target.value))}
          error={!!errors.days}
          helperText={errors.days}
          margin="normal"
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
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

export default EditAuditLogRetentionPolicyModal;
