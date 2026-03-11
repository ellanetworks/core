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
import { updateOperatorID } from "@/queries/operator";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditOperatorIdModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    mcc: string;
    mnc: string;
  };
}

const schema = yup.object().shape({
  mcc: yup
    .string()
    .matches(/^\d{3}$/, "MCC must be a 3 decimal digit")
    .required("MCC is required"),
  mnc: yup
    .string()
    .matches(/^\d{2,3}$/, "MNC must be a 2 or 3 decimal digit")
    .required("MNC is required"),
});

const EditOperatorIdModal: React.FC<EditOperatorIdModalProps> = ({
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
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues(initialData);
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
      await updateOperatorID(accessToken, formValues.mcc, formValues.mnc);
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({ message: `Failed to update operator ID: ${errorMessage}` });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-operator-id-modal-title"
      aria-describedby="edit-operator-id-modal-description"
    >
      <DialogTitle id="edit-operator-id-modal-title">
        Edit Operator ID
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
          The Operator ID is a combination of Mobile Country Code (MCC) and
          Mobile Network Code (MNC). The Operator ID is used to uniquely
          identify the operator in the network.
        </DialogContentText>
        <TextField
          fullWidth
          label="MCC"
          value={formValues.mcc}
          onChange={(e) => handleChange("mcc", e.target.value)}
          onBlur={() => handleBlur("mcc")}
          error={touched.mcc && !!errors.mcc}
          helperText={touched.mcc ? errors.mcc : ""}
          margin="normal"
          autoFocus
        />
        <TextField
          fullWidth
          label="MNC"
          value={formValues.mnc}
          onChange={(e) => handleChange("mnc", e.target.value)}
          onBlur={() => handleBlur("mnc")}
          error={touched.mnc && !!errors.mnc}
          helperText={touched.mnc ? errors.mnc : ""}
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

export default EditOperatorIdModal;
