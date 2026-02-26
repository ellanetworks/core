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
  updateFlowReportsRetentionPolicy,
  type FlowReportsRetentionPolicy,
} from "@/queries/flow_reports";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditFlowReportsRetentionPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: FlowReportsRetentionPolicy;
}

const EditFlowReportsRetentionPolicyModal: React.FC<
  EditFlowReportsRetentionPolicyModalProps
> = ({ open, onClose, onSuccess, initialData }) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  if (!authReady || !accessToken) {
    navigate("/login");
  }

  const [formValues, setFormValues] = useState(initialData);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({ days: initialData.days });
      setErrors({});
    }
  }, [open, initialData]);

  const handleChange = (field: string, value: string | number) => {
    setFormValues((prev) => ({ ...prev, [field]: value }));
  };

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateFlowReportsRetentionPolicy(accessToken, formValues.days);
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update flow reports retention policy: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-flow-reports-retention-policy-modal-title"
      aria-describedby="edit-flow-reports-retention-policy-modal-description"
    >
      <DialogTitle>Edit Flow Reports Retention Policy</DialogTitle>
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
          Set the number of days to retain flow report data. After this period,
          data will be automatically deleted.
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

export default EditFlowReportsRetentionPolicyModal;
