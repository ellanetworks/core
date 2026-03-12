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
import { updateRadioEventRetentionPolicy } from "@/queries/radio_events";
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

interface EditRadioEventRetentionPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialDays: number;
}

const EditRadioEventRetentionPolicyModal: React.FC<
  EditRadioEventRetentionPolicyModalProps
> = ({ open, onClose, onSuccess, initialDays }) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (!authReady || !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState({ days: initialDays });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [isValid, setIsValid] = useState(true);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({ days: initialDays });
      setErrors({});
      setIsValid(true);
    }
  }, [open, initialDays]);

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
      await updateRadioEventRetentionPolicy(accessToken, formValues.days);
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update radio event retention policy: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-network-log-retention-policy-modal-title"
      aria-describedby="edit-network-log-retention-policy-modal-description"
    >
      <DialogTitle id="edit-network-log-retention-policy-modal-title">
        Edit Network Log Retention Policy
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
          Set the number of days to retain radio events. After this period, logs
          will be automatically deleted.
        </DialogContentText>
        {formValues.days < initialDays && (
          <Alert severity="warning" sx={{ mb: 2 }}>
            Reducing the retention period will permanently delete radio events
            older than {formValues.days} day{formValues.days !== 1 ? "s" : ""}.
          </Alert>
        )}
        <TextField
          fullWidth
          required
          type="number"
          label="Days"
          value={formValues.days}
          onChange={(e) => handleChange("days", Number(e.target.value))}
          error={!!errors.days}
          helperText={errors.days || "1 to 3650 days"}
          margin="normal"
          autoFocus
          slotProps={{
            input: {
              inputProps: { min: 1, max: 3650 },
            },
          }}
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

export default EditRadioEventRetentionPolicyModal;
