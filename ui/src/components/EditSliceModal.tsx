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
import { updateSlice, type APISlice } from "@/queries/slices";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditSliceModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APISlice;
}

const schema = yup.object().shape({
  sst: yup
    .number()
    .min(0, "SST must be between 0 and 255")
    .max(255, "SST must be between 0 and 255")
    .required("SST is required"),
  sd: yup
    .string()
    .matches(/^([0-9a-fA-F]{6})?$/, "SD must be a 6-digit hex string"),
});

const EditSliceModal: React.FC<EditSliceModalProps> = ({
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
    name: initialData.name,
    sst: initialData.sst,
    sd: initialData.sd,
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({
        name: initialData.name,
        sst: initialData.sst,
        sd: initialData.sd,
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
  }, [validateForm, formValues]);

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateSlice(
        accessToken,
        formValues.name,
        formValues.sst,
        formValues.sd,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update network slice: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-slice-modal-title"
    >
      <DialogTitle id="edit-slice-modal-title">Edit Network Slice</DialogTitle>
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
          label="SST"
          type="number"
          value={formValues.sst}
          onChange={(e) => handleChange("sst", Number(e.target.value))}
          onBlur={() => handleBlur("sst")}
          error={!!errors.sst && touched.sst}
          helperText={
            touched.sst && errors.sst
              ? errors.sst
              : "Slice/Service Type (0–255)"
          }
          margin="normal"
          autoFocus
          inputProps={{ min: 0, max: 255 }}
        />
        <TextField
          fullWidth
          label="SD (optional)"
          value={formValues.sd}
          onChange={(e) => handleChange("sd", e.target.value)}
          onBlur={() => handleBlur("sd")}
          error={!!errors.sd && touched.sd}
          helperText={
            touched.sd && errors.sd
              ? errors.sd
              : "Slice Differentiator — 6 hex digits (e.g. 010203)"
          }
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

export default EditSliceModal;
