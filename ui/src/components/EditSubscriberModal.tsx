import React, { useState, useEffect } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
  Alert,
  Collapse,
  Select,
  MenuItem,
  InputLabel,
  FormControl,
} from "@mui/material";
import { updateSubscriber } from "@/queries/subscribers";
import {
  listPolicies,
  type APIPolicy,
  type ListPoliciesResponse,
} from "@/queries/policies";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditSubscriberModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    imsi: string;
    policyName: string;
  };
}

const EditSubscriberModal: React.FC<EditSubscriberModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  if (!authReady || !accessToken) {
    navigate("/login");
  }

  const [formValues, setFormValues] = useState(initialData);
  const [policies, setPolicies] = useState<string[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (!accessToken) return;

    const fetchPoliciesPaginated = async () => {
      try {
        const page: ListPoliciesResponse = await listPolicies(
          accessToken,
          1,
          100,
        );
        setPolicies((page.items ?? []).map((p: APIPolicy) => p.name));
      } catch (error) {
        console.error("Failed to fetch policies:", error);
      }
    };

    if (open) {
      fetchPoliciesPaginated();
      setFormValues(initialData);
      setErrors({});
    }
  }, [open, initialData, accessToken]);

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
      await updateSubscriber(
        accessToken,
        formValues.imsi,
        formValues.policyName,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({ message: `Failed to update subscriber: ${errorMessage}` });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-subscriber-modal-title"
      aria-describedby="edit-subscriber-modal-description"
    >
      <DialogTitle>Edit Subscriber</DialogTitle>
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
          label="IMSI"
          value={formValues.imsi}
          margin="normal"
          disabled
        />

        <FormControl fullWidth margin="normal">
          <InputLabel id="policy-select-label">Policy Name</InputLabel>
          <Select
            labelId="policy-select-label"
            label="Policy Name"
            value={formValues.policyName}
            onChange={(e) => handleChange("policyName", e.target.value)}
            error={!!errors.policyName}
          >
            {policies.map((policy) => (
              <MenuItem key={policy} value={policy}>
                {policy}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
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

export default EditSubscriberModal;
