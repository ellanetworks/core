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
  listProfiles,
  type APIProfile,
  type ListProfilesResponse,
} from "@/queries/profiles";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditSubscriberModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    imsi: string;
    profileName: string;
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

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState(initialData);
  const [profiles, setProfiles] = useState<string[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (!accessToken) return;

    const fetchProfilesPaginated = async () => {
      try {
        const page: ListProfilesResponse = await listProfiles(
          accessToken,
          1,
          100,
        );
        setProfiles((page.items ?? []).map((p: APIProfile) => p.name));
      } catch (error) {
        console.error("Failed to fetch profiles:", error);
      }
    };

    if (open) {
      fetchProfilesPaginated();
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
        formValues.profileName,
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
      fullWidth
      maxWidth="sm"
    >
      <DialogTitle id="edit-subscriber-modal-title">
        Edit Subscriber
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

        <TextField
          fullWidth
          label="IMSI"
          value={formValues.imsi}
          margin="normal"
          disabled
        />

        <FormControl fullWidth margin="normal">
          <InputLabel id="profile-select-label">Profile</InputLabel>
          <Select
            labelId="profile-select-label"
            label="Profile"
            value={formValues.profileName}
            onChange={(e) => handleChange("profileName", e.target.value)}
            error={!!errors.profileName}
            autoFocus
          >
            {profiles.map((profile) => (
              <MenuItem key={profile} value={profile}>
                {profile}
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
