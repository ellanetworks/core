import React, { useCallback, useState, useEffect } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  TextField,
  Button,
  Alert,
  Collapse,
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { updateBGPSettings } from "@/queries/bgp";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditBGPSettingsModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    enabled: boolean;
    localAS: string;
    routerID: string;
    listenAddress: string;
  };
}

const schema = yup.object().shape({
  localAS: yup
    .string()
    .matches(/^\d+$/, "Local AS must be a number")
    .required("Local AS is required")
    .test("range", "Local AS must be between 1 and 4294967295", (val) => {
      const n = Number(val);
      return n >= 1 && n <= 4294967295;
    }),
  routerID: yup
    .string()
    .test("ipv4", "Router ID must be a valid IPv4 address or empty", (val) => {
      if (!val || val === "") return true;
      return /^(\d{1,3}\.){3}\d{1,3}$/.test(val);
    }),
  listenAddress: yup
    .string()
    .required("Listen address is required")
    .test(
      "host-port",
      "Listen address must be in host:port or :port format",
      (val) => {
        if (!val) return false;
        const match = val.match(/^(.*):(\d+)$/);
        if (!match) return false;
        const port = Number(match[2]);
        return port >= 1 && port <= 65535;
      },
    ),
});

const EditBGPSettingsModal: React.FC<EditBGPSettingsModalProps> = ({
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

  const [formValues, setFormValues] = useState({
    localAS: initialData.localAS,
    routerID: initialData.routerID,
    listenAddress: initialData.listenAddress,
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({
        localAS: initialData.localAS,
        routerID: initialData.routerID,
        listenAddress: initialData.listenAddress,
      });
      setErrors({});
      setTouched({});
    }
  }, [open, initialData]);

  const handleChange = (field: string, value: string) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));
    validateField(field, value);
  };

  const handleBlur = (field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const validateField = async (field: string, value: string) => {
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
      setIsValid(true);
    } catch {
      setIsValid(false);
    }
  }, [formValues]);

  useEffect(() => {
    validateForm();
  }, [validateForm]);

  const handleSubmit = async () => {
    if (!accessToken) return;

    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateBGPSettings(accessToken, {
        enabled: initialData.enabled,
        localAS: Number(formValues.localAS),
        routerID: formValues.routerID,
        listenAddress: formValues.listenAddress,
      });
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update BGP settings: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-bgp-settings-modal-title"
      aria-describedby="edit-bgp-settings-modal-description"
    >
      <DialogTitle id="edit-bgp-settings-modal-title">
        Edit BGP Settings
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
        <DialogContentText id="edit-bgp-settings-modal-description">
          Configure the local BGP speaker. Changes may require the BGP speaker
          to restart.
        </DialogContentText>
        <TextField
          fullWidth
          label="Local AS"
          type="number"
          value={formValues.localAS}
          onChange={(e) => handleChange("localAS", e.target.value)}
          onBlur={() => handleBlur("localAS")}
          error={touched.localAS && !!errors.localAS}
          helperText={touched.localAS ? errors.localAS : ""}
          margin="normal"
          autoFocus
        />
        <TextField
          fullWidth
          label="Router ID"
          value={formValues.routerID}
          onChange={(e) => handleChange("routerID", e.target.value)}
          onBlur={() => handleBlur("routerID")}
          error={touched.routerID && !!errors.routerID}
          helperText={touched.routerID ? errors.routerID : ""}
          margin="normal"
          placeholder="e.g. 10.0.0.1"
        />
        <TextField
          fullWidth
          label="Listen Address"
          value={formValues.listenAddress}
          onChange={(e) => handleChange("listenAddress", e.target.value)}
          onBlur={() => handleBlur("listenAddress")}
          error={touched.listenAddress && !!errors.listenAddress}
          helperText={touched.listenAddress ? errors.listenAddress : ""}
          margin="normal"
          placeholder=":179"
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

export default EditBGPSettingsModal;
