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
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  SelectChangeEvent,
} from "@mui/material";
import { updateUser, RoleID, APIUser, roleIDToLabel } from "@/queries/users";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";

interface EditUserModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APIUser;
}

interface FormValues {
  email: string;
  role: RoleID;
}

const EditUserModal: React.FC<EditUserModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const router = useRouter();
  const { accessToken, authReady } = useAuth();

  if (!authReady || !accessToken) router.push("/login");

  const [formValues, setFormValues] = useState<FormValues>({
    email: "",
    role: RoleID.ReadOnly,
  });

  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({
        email: initialData.email,
        role: initialData.role_id,
      });
    }
  }, [open, initialData]);

  const handleChange = (event: SelectChangeEvent) => {
    setFormValues((prev) => ({
      ...prev,
      role: parseInt(event.target.value, 10) as RoleID,
    }));
  };

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateUser(accessToken, formValues.email, formValues.role);
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const message =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({ message: `Failed to update user: ${message}` });
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
      <DialogTitle>Edit User</DialogTitle>
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
        <FormControl fullWidth margin="normal">
          <InputLabel id="role-select-label">Role</InputLabel>
          <Select
            labelId="role-select-label"
            id="role-select"
            value={formValues.role.toString()}
            label="Role"
            onChange={handleChange}
          >
            <MenuItem value={RoleID.Admin.toString()}>
              {roleIDToLabel(RoleID.Admin)}
            </MenuItem>
            <MenuItem value={RoleID.NetworkManager.toString()}>
              {roleIDToLabel(RoleID.NetworkManager)}
            </MenuItem>
            <MenuItem value={RoleID.ReadOnly.toString()}>
              {roleIDToLabel(RoleID.ReadOnly)}
            </MenuItem>
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

export default EditUserModal;
