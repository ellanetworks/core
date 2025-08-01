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
import { updateDataNetwork } from "@/queries/data_networks";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";
import { DataNetwork } from "@/types/types";

interface EditDataNetworkModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: DataNetwork;
}

const EditDataNetworkModal: React.FC<EditDataNetworkModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
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
        name: initialData.name,
        ipPool: initialData.ipPool,
        dns: initialData.dns,
        mtu: initialData.mtu,
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
      await updateDataNetwork(
        cookies.user_token,
        formValues.name,
        formValues.ipPool,
        formValues.dns,
        formValues.mtu,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({ message: `Failed to update data network: ${errorMessage}` });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-data-network-modal-title"
      aria-describedby="edit-data-network-modal-description"
    >
      <DialogTitle>Edit Data Network</DialogTitle>
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
          label="Name"
          value={formValues.name}
          margin="normal"
          disabled
        />
        <TextField
          fullWidth
          label="IP Pool"
          value={formValues.ipPool}
          onChange={(e) => handleChange("ipPool", e.target.value)}
          error={!!errors.ipPool}
          helperText={errors.ipPool}
          margin="normal"
        />
        <TextField
          fullWidth
          label="DNS"
          value={formValues.dns}
          onChange={(e) => handleChange("dns", e.target.value)}
          error={!!errors.dns}
          helperText={errors.dns}
          margin="normal"
        />
        <TextField
          fullWidth
          label="MTU"
          type="number"
          value={formValues.mtu}
          onChange={(e) => handleChange("mtu", Number(e.target.value))}
          error={!!errors.mtu}
          helperText={errors.mtu}
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

export default EditDataNetworkModal;
