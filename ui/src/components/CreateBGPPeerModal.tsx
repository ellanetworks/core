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
import { createBGPPeer } from "@/queries/bgp";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

const ipv4Regex =
  /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;

const schema = yup.object().shape({
  address: yup
    .string()
    .required("Address is required")
    .matches(ipv4Regex, "Must be a valid IPv4 address"),
  remoteAS: yup
    .number()
    .required("Remote AS is required")
    .min(1, "Must be at least 1")
    .max(4294967295, "Must be at most 4294967295"),
  holdTime: yup
    .number()
    .required("Hold time is required")
    .min(3, "Must be at least 3")
    .max(65535, "Must be at most 65535"),
  password: yup.string(),
  description: yup.string(),
});

interface CreateBGPPeerModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

type FormValues = {
  address: string;
  remoteAS: number;
  holdTime: number;
  password: string;
  description: string;
};

const CreateBGPPeerModal: React.FC<CreateBGPPeerModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState<FormValues>({
    address: "",
    remoteAS: 64512,
    holdTime: 90,
    password: "",
    description: "",
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const handleChange = (field: string, value: string | number) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));
    validateField(field, value);
  };

  const handleBlur = (field: string) => {
    setTouched((prev) => ({
      ...prev,
      [field]: true,
    }));
  };

  const validateField = async (field: string, value: string | number) => {
    try {
      await schema.validateAt(field, { ...formValues, [field]: value });
      setErrors((prev) => {
        const next = { ...prev };
        delete next[field];
        return next;
      });
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
  }, [formValues, validateForm]);

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });
    try {
      await createBGPPeer(accessToken, {
        address: formValues.address,
        remoteAS: formValues.remoteAS,
        holdTime: formValues.holdTime,
        password: formValues.password || undefined,
        description: formValues.description || undefined,
      });
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to create BGP peer: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="create-bgp-peer-modal-title"
    >
      <DialogTitle id="create-bgp-peer-modal-title">
        Create BGP Peer
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
          label="Address"
          value={formValues.address}
          onChange={(e) => handleChange("address", e.target.value)}
          onBlur={() => handleBlur("address")}
          error={!!errors.address && touched.address}
          helperText={touched.address ? errors.address : ""}
          margin="normal"
          autoFocus
        />
        <TextField
          fullWidth
          label="Remote AS"
          type="number"
          value={formValues.remoteAS}
          onChange={(e) => handleChange("remoteAS", Number(e.target.value))}
          onBlur={() => handleBlur("remoteAS")}
          error={!!errors.remoteAS && touched.remoteAS}
          helperText={touched.remoteAS ? errors.remoteAS : ""}
          margin="normal"
        />
        <TextField
          fullWidth
          label="Hold Time"
          type="number"
          value={formValues.holdTime}
          onChange={(e) => handleChange("holdTime", Number(e.target.value))}
          onBlur={() => handleBlur("holdTime")}
          error={!!errors.holdTime && touched.holdTime}
          helperText={touched.holdTime ? errors.holdTime : ""}
          margin="normal"
        />
        <TextField
          fullWidth
          label="Password"
          type="password"
          value={formValues.password}
          onChange={(e) => handleChange("password", e.target.value)}
          onBlur={() => handleBlur("password")}
          margin="normal"
        />
        <TextField
          fullWidth
          label="Description"
          value={formValues.description}
          onChange={(e) => handleChange("description", e.target.value)}
          onBlur={() => handleBlur("description")}
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
          {loading ? "Creating..." : "Create"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default CreateBGPPeerModal;
