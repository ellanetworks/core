import React, { useCallback, useState, useEffect } from "react";
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
import * as yup from "yup";
import { ValidationError } from "yup";
import { updateDataNetwork, APIDataNetwork } from "@/queries/data_networks";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditDataNetworkModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APIDataNetwork;
}

const schema = yup.object().shape({
  ip_pool: yup
    .string()
    .matches(
      /^(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\/\d{1,2}$/,
      "Must be a valid IP pool (e.g., 10.45.0.0/22)",
    )
    .required("IP Pool is required"),
  dns: yup
    .string()
    .matches(
      /^(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)$/,
      "Must be a valid IP address",
    )
    .required("DNS is required"),
  mtu: yup.number().min(1).max(65535).required("MTU is required"),
});

const EditDataNetworkModal: React.FC<EditDataNetworkModalProps> = ({
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
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({
        name: initialData.name,
        ip_pool: initialData.ip_pool,
        dns: initialData.dns,
        mtu: initialData.mtu,
      });
      setErrors({});
      setTouched({});
    }
  }, [open, initialData]);

  const handleChange = (field: string, value: string | number) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));
    validateField(field, value);
  };

  const handleBlur = (field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const validateField = async (field: string, value: string | number) => {
    try {
      const fieldSchema = yup.reach(schema, field) as yup.Schema<unknown>;
      await fieldSchema.validate(value);
      setErrors((prev) => ({ ...prev, [field]: "" }));
    } catch (err) {
      if (err instanceof ValidationError) {
        setErrors((prev) => ({ ...prev, [field]: err.message }));
      }
    }
  };

  const validateForm = useCallback(async () => {
    try {
      await schema.validate(formValues, { abortEarly: false });
      setErrors({});
      setIsValid(true);
    } catch (err) {
      if (err instanceof ValidationError) {
        const validationErrors = err.inner.reduce(
          (acc, curr) => {
            acc[curr.path!] = curr.message;
            return acc;
          },
          {} as Record<string, string>,
        );
        setErrors(validationErrors);
      }
      setIsValid(false);
    }
  }, [formValues]);

  useEffect(() => {
    validateForm();
  }, [validateForm, formValues]);

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateDataNetwork(
        accessToken,
        formValues.name,
        formValues.ip_pool,
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
      <DialogTitle id="edit-data-network-modal-title">
        Edit Data Network
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
          label="Name"
          value={formValues.name}
          margin="normal"
          disabled
        />
        <TextField
          fullWidth
          label="IP Pool"
          value={formValues.ip_pool}
          onChange={(e) => handleChange("ip_pool", e.target.value)}
          onBlur={() => handleBlur("ip_pool")}
          error={!!errors.ip_pool && touched.ip_pool}
          helperText={touched.ip_pool ? errors.ip_pool : ""}
          margin="normal"
          autoFocus
        />
        <TextField
          fullWidth
          label="DNS"
          value={formValues.dns}
          onChange={(e) => handleChange("dns", e.target.value)}
          onBlur={() => handleBlur("dns")}
          error={!!errors.dns && touched.dns}
          helperText={touched.dns ? errors.dns : ""}
          margin="normal"
        />
        <TextField
          fullWidth
          label="MTU"
          type="number"
          value={formValues.mtu}
          onChange={(e) => handleChange("mtu", Number(e.target.value))}
          onBlur={() => handleBlur("mtu")}
          error={!!errors.mtu && touched.mtu}
          helperText={touched.mtu ? errors.mtu : ""}
          margin="normal"
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={!isValid || loading}
        >
          {loading ? "Updating..." : "Update"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default EditDataNetworkModal;
