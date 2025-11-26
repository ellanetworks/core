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
import { updateRadioEventRetentionPolicy } from "@/queries/radio_events";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";

interface EditRadioEventRetentionPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialDays: number;
}

const EditRadioEventRetentionPolicyModal: React.FC<
  EditRadioEventRetentionPolicyModalProps
> = ({ open, onClose, onSuccess, initialDays }) => {
  const router = useRouter();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (!authReady || !accessToken) router.push("/login");
  }, [authReady, accessToken, router]);

  const [formValues, setFormValues] = useState({ days: initialDays });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({ days: initialDays });
      setErrors({});
    }
  }, [open, initialDays]);

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
      <DialogTitle>Edit Network Log Retention Policy</DialogTitle>
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

export default EditRadioEventRetentionPolicyModal;
