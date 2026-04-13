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
import { createSlice } from "@/queries/slices";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface CreateSliceModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object().shape({
  name: yup
    .string()
    .max(255, "Name must be 255 characters or less")
    .required("Name is required"),
  sst: yup
    .number()
    .min(0, "SST must be between 0 and 255")
    .max(255, "SST must be between 0 and 255")
    .required("SST is required"),
  sd: yup
    .string()
    .matches(/^([0-9a-fA-F]{6})?$/, "SD must be a 6-digit hex string"),
});

const CreateSliceModal: React.FC<CreateSliceModalProps> = ({
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

  const [formValues, setFormValues] = useState({
    name: "",
    sst: 1,
    sd: "",
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
  }, [validateForm, formValues]);

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });
    try {
      await createSlice(
        accessToken,
        formValues.name,
        formValues.sst,
        formValues.sd,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      let errorMessage = "Unknown error occurred.";
      if (error instanceof Error) {
        errorMessage = error.message;
      }

      setAlert({
        message: `Failed to create network slice: ${errorMessage}`,
      });
      console.error("Failed to create network slice:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="create-slice-modal-title"
    >
      <DialogTitle id="create-slice-modal-title">
        Create Network Slice
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
          onChange={(e) => handleChange("name", e.target.value)}
          onBlur={() => handleBlur("name")}
          error={!!errors.name && touched.name}
          helperText={touched.name ? errors.name : ""}
          margin="normal"
          autoFocus
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
          slotProps={{ htmlInput: { min: 0, max: 255 } }}
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
          {loading ? "Creating..." : "Create"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default CreateSliceModal;
