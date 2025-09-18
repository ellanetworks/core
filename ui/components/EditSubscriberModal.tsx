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
import { listPolicies } from "@/queries/policies";
import { useRouter } from "next/navigation";
import { Policy } from "@/types/types";
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
  const router = useRouter();
  const { accessToken, authReady } = useAuth();

  if (!authReady || !accessToken) {
    router.push("/login");
  }
  const [formValues, setFormValues] = useState(initialData);
  const [policies, setPolicies] = useState<string[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (!accessToken) return;
    const fetchPolicies = async () => {
      try {
        const policyData = await listPolicies(accessToken);
        setPolicies(policyData.map((policy: Policy) => policy.name));
      } catch (error) {
        console.error("Failed to fetch policies:", error);
      }
    };

    if (open) {
      fetchPolicies();
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
      let errorMessage = "Unknown error occurred.";
      if (error instanceof Error) {
        errorMessage = error.message;
      }
      setAlert({
        message: `Failed to get subscriber: ${errorMessage}`,
      });
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
          <InputLabel id="demo-simple-select-label">Policy Name</InputLabel>
          <Select
            value={formValues.policyName}
            onChange={(e) => handleChange("policyName", e.target.value)}
            error={!!errors.policyName}
            label={"Policy Name"}
            labelId="demo-simple-select-label"
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
