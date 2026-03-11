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
import { updateOperatorHomeNetwork } from "@/queries/operator";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditOperatorHomeNetworkModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object().shape({
  privateKey: yup
    .string()
    .matches(
      /^[a-fA-F0-9]{64}$/,
      "Private Key must be a 64-character hexadecimal string.",
    )
    .required("Private Key is required."),
});

const EditOperatorHomeNetworkModal: React.FC<
  EditOperatorHomeNetworkModalProps
> = ({ open, onClose, onSuccess }) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState<{ privateKey: string }>({
    privateKey: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({ privateKey: "" });
      setErrors({});
      setTouched({});
    }
  }, [open]);

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
      const fieldSchema = yup.reach(schema, field) as yup.Schema<unknown>;
      await fieldSchema.validate(value);
      setErrors((prev) => ({ ...prev, [field]: "" }));
    } catch (err) {
      if (err instanceof yup.ValidationError) {
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
      await updateOperatorHomeNetwork(accessToken, formValues.privateKey);
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update operator home network information: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-operator-home-network-modal-title"
      aria-describedby="edit-operator-home-network-modal-description"
    >
      <DialogTitle id="edit-operator-home-network-modal-title">
        Edit Operator Home Network Information
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
        <DialogContentText id="alert-dialog-slide-description">
          The Home Network Private Key ensures IMSI privacy. User Equipment (UE)
          devices will use the public key to encrypt the IMSI before sending it
          to the network. The network will then use the private key to decrypt
          the IMSI.
        </DialogContentText>
        <Alert severity="warning" sx={{ mb: 2 }}>
          Changing the private key will affect all subscriber IMSI encryption.
          Ensure all UE devices are updated with the corresponding public key.
        </Alert>
        <TextField
          fullWidth
          label="Private Key"
          value={formValues.privateKey}
          onChange={(e) => handleChange("privateKey", e.target.value)}
          onBlur={() => handleBlur("privateKey")}
          error={touched.privateKey && !!errors.privateKey}
          helperText={touched.privateKey ? errors.privateKey : ""}
          margin="normal"
          autoFocus
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

export default EditOperatorHomeNetworkModal;
