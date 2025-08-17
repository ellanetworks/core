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
import { updateSubscriberLogRetentionPolicy } from "@/queries/subscriber_logs";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";
import { SubscriberLogRetentionPolicy } from "@/types/types";

interface EditSubscriberLogRetentionPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: SubscriberLogRetentionPolicy;
}

const EditSubscriberLogRetentionPolicyModal: React.FC<
  EditSubscriberLogRetentionPolicyModalProps
> = ({ open, onClose, onSuccess, initialData }) => {
  const router = useRouter();
  const [cookies, ,] = useCookies(["user_token"]);

  if (!cookies.user_token) {
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
    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateSubscriberLogRetentionPolicy(
        cookies.user_token,
        formValues.days,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update subscriber log retention policy: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-subscriber-log-retention-policy-modal-title"
      aria-describedby="edit-subscriber-log-retention-policy-modal-description"
    >
      <DialogTitle>Edit Subscriber Log Retention Policy</DialogTitle>
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
          Set the number of days to retain subscriber logs. After this period,
          logs will be automatically deleted.
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

export default EditSubscriberLogRetentionPolicyModal;
