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
  InputAdornment,
} from "@mui/material";
import * as yup from "yup";
import { updateOperatorSlice } from "@/queries/operator";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditOperatorSliceModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    sst: number;
    sd?: string | null; // optional, stored WITHOUT 0x
  };
}

// Final-form validation (allow empty or exactly 6 hex digits)
const schema = yup.object({
  sst: yup
    .number()
    .typeError("SST is required")
    .required("SST is required")
    .integer("SST must be an integer")
    .min(0, "SST must be at least 0")
    .max(255, "SST must be at most 255"),
  sd: yup
    .string()
    .transform((v) =>
      typeof v === "string" && v.trim() === "" ? undefined : v,
    )
    .optional()
    .matches(/^$|^[0-9a-fA-F]{6}$/, {
      message: "SD must be exactly 6 hex digits (e.g., 012030)",
      excludeEmptyString: true,
    }),
});

// Normalize for API: return undefined when empty, else lowercase 6-hex (no 0x)
function normalizeSd(sd?: string | null): string | undefined {
  if (!sd) return undefined;
  const v = sd.trim();
  if (!v) return undefined;
  return v.toLowerCase();
}

// Live sanitizer for the SD input (runs on every change)
function sanitizeSdInput(raw: string): string {
  let v = raw.trim();

  // Strip any leading 0x/0X (user paste)
  if (v.startsWith("0x") || v.startsWith("0X")) v = v.slice(2);

  // Keep hex only
  v = v.replace(/[^0-9a-fA-F]/g, "");

  // Limit to 6 chars
  if (v.length > 6) v = v.slice(0, 6);

  return v;
}

const EditOperatorSliceModal: React.FC<EditOperatorSliceModalProps> = ({
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

  const [formValues, setFormValues] = useState<{
    sst: number | string;
    sd: string; // plain hex with NO 0x
  }>({
    sst: initialData.sst,
    sd: (initialData.sd ?? "").replace(/^0x/i, "").slice(0, 6),
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({
        sst: initialData.sst,
        sd: (initialData.sd ?? "").replace(/^0x/i, "").slice(0, 6),
      });
      setErrors({});
      setTouched({});
      setAlert({ message: "" });
    }
  }, [open, initialData]);

  const handleChange = (field: "sst" | "sd", value: string) => {
    const sanitized = field === "sd" ? sanitizeSdInput(value) : value;
    setFormValues((prev) => ({
      ...prev,
      [field]: sanitized,
    }));
    validateField(field, field === "sst" ? Number(sanitized) : sanitized);
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
      if (err instanceof yup.ValidationError) {
        setErrors((prev) => ({ ...prev, [field]: err.message }));
      }
    }
  };

  const validateForm = useCallback(async () => {
    try {
      const prepared = {
        sst:
          typeof formValues.sst === "string"
            ? Number(formValues.sst)
            : formValues.sst,
        sd: formValues.sd,
      };
      await schema.validate(prepared, { abortEarly: false });
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

    const sstNum =
      typeof formValues.sst === "string"
        ? Number(formValues.sst)
        : formValues.sst;
    const sdNorm = normalizeSd(formValues.sd); // undefined or plain 6-hex lowercase

    try {
      await updateOperatorSlice(accessToken, sstNum, sdNorm);
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update operator slice information: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-operator-slice-modal-title"
      aria-describedby="edit-operator-slice-modal-description"
    >
      <DialogTitle id="edit-operator-slice-modal-title">
        Edit Operator Slice Information
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

        <DialogContentText id="edit-operator-slice-modal-description">
          The Slice Information identifies the network slice. Ella Core supports
          one slice. SST is an 8-bit value (0–255). SD is optional; when present
          it must be 24-bit hex.
        </DialogContentText>

        <TextField
          fullWidth
          label="SST"
          value={formValues.sst}
          onChange={(e) => handleChange("sst", e.target.value)}
          onBlur={() => handleBlur("sst")}
          error={touched.sst && !!errors.sst}
          helperText={
            (touched.sst && errors.sst) || "Enter an integer between 0 and 255."
          }
          margin="normal"
          inputProps={{
            inputMode: "numeric",
            pattern: "[0-9]*",
            min: 0,
            max: 255,
          }}
        />

        <TextField
          fullWidth
          label="SD (optional)"
          value={formValues.sd}
          onChange={(e) => handleChange("sd", e.target.value)}
          onBlur={() => handleBlur("sd")}
          error={touched.sd && !!errors.sd}
          helperText={(touched.sd && errors.sd) || "Enter 6 hex digits."}
          margin="normal"
          placeholder="012030"
          inputProps={{ spellCheck: false, inputMode: "text" }}
          InputProps={{
            startAdornment: (
              <InputAdornment position="start">0x</InputAdornment>
            ),
          }}
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

export default EditOperatorSliceModal;
