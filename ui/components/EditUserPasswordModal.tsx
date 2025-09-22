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
} from "@mui/material";
import { updateUserPassword } from "@/queries/users";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";

interface EditUserPasswordModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    email: string;
  };
}

interface FormValues {
  email: string;
  password: string;
}

const EditUserPasswordModal: React.FC<EditUserPasswordModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const router = useRouter();
  const { accessToken, authReady } = useAuth();
  if (!authReady || !accessToken) router.push("/login");

  const [formValues, setFormValues] = useState<FormValues>({
    email: initialData.email,
    password: "",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({
        email: initialData.email,
        password: "",
      });
      setErrors({});
    }
  }, [open, initialData]);

  const handleChange = (field: keyof FormValues, value: string) => {
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
      await updateUserPassword(
        accessToken,
        formValues.email,
        formValues.password,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      let errorMessage = "Unknown error occurred.";
      if (error instanceof Error) {
        errorMessage = error.message;
      }
      setAlert({ message: `Failed to update user: ${errorMessage}` });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-user-modal-title"
      aria-describedby="edit-user-modal-description"
    >
      <DialogTitle>Edit User Password</DialogTitle>
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
          label="Email"
          value={formValues.email}
          margin="normal"
          disabled
        />
        <TextField
          fullWidth
          label="New Password"
          type="password"
          value={formValues.password}
          onChange={(e) => handleChange("password", e.target.value)}
          error={!!errors.password}
          helperText={errors.password}
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

export default EditUserPasswordModal;
