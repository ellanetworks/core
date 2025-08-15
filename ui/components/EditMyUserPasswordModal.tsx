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
import { updateMyUserPassword } from "@/queries/users";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface EditMyUserPasswordModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

interface FormValues {
  password: string;
}

const EditMyUserPasswordModal: React.FC<EditMyUserPasswordModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const router = useRouter();
  const [cookies] = useCookies(["user_token"]);

  if (!cookies.user_token) {
    router.push("/login");
  }

  const [formValues, setFormValues] = useState<FormValues>({
    password: "",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({
        password: "",
      });
      setErrors({});
    }
  }, [open]);

  const handleChange = (field: keyof FormValues, value: string) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));
  };

  const handleSubmit = async () => {
    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateMyUserPassword(cookies.user_token, formValues.password);
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
      <DialogTitle>Change My Password</DialogTitle>
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

export default EditMyUserPasswordModal;
