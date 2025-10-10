import React, { useState, useEffect } from "react";
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
import { updateOperatorSlice } from "@/queries/operator";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";

interface EditOperatorSliceModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: {
    sst: number;
    sd?: string | null; // <-- make optional
  };
}

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
    .matches(/^$|^(0x)?[0-9a-fA-F]{6}$/, {
      message: "SD must be 6 hex digits (e.g., 0x012030 or 012030)",
      excludeEmptyString: true,
    }),
});

function normalizeSd(sd?: string | null): string | undefined {
  if (!sd) return undefined;
  let v = sd.trim();
  if (v === "") return undefined;
  if (v.startsWith("0x") || v.startsWith("0X")) v = v.slice(2);
  return `0x${v.toLowerCase()}`;
}

const EditOperatorSliceModal: React.FC<EditOperatorSliceModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const router = useRouter();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (!authReady || !accessToken) {
      router.push("/login");
    }
  }, [authReady, accessToken, router]);

  const [formValues, setFormValues] = useState<{
    sst: number | string;
    sd: string;
  }>({
    sst: initialData.sst,
    sd: initialData.sd ?? "", // keep as controlled string
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({
        sst: initialData.sst,
        sd: initialData.sd ?? "",
      });
      setErrors({});
      setAlert({ message: "" });
    }
  }, [open, initialData]);

  const handleChange = (field: "sst" | "sd", value: string) => {
    setFormValues((prev) => ({
      ...prev,
      ...(field === "sst"
        ? { sst: value } // keep as string for the input; convert later
        : { sd: value }), // keep sd as string (not parseInt!)
    }));
    setErrors((prev) => ({ ...prev, [field]: "" }));
  };

  const validate = async (): Promise<boolean> => {
    try {
      // Convert sst to number before validating
      const prepared = {
        sst:
          typeof formValues.sst === "string"
            ? Number(formValues.sst)
            : formValues.sst,
        sd: formValues.sd,
      };
      await schema.validate(prepared, { abortEarly: false });
      setErrors({});
      return true;
    } catch (err) {
      if (err instanceof yup.ValidationError) {
        const validationErrors: Record<string, string> = {};
        err.inner.forEach((e) => {
          if (e.path) validationErrors[e.path] = e.message;
        });
        setErrors(validationErrors);
      }
      return false;
    }
  };

  const handleSubmit = async () => {
    if (!accessToken) return;
    const isValid = await validate();
    if (!isValid) return;

    setLoading(true);
    setAlert({ message: "" });

    const sstNum =
      typeof formValues.sst === "string"
        ? Number(formValues.sst)
        : formValues.sst;
    const sdNorm = normalizeSd(formValues.sd);

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
          error={!!errors.sst}
          helperText={errors.sst || "Enter an integer between 0 and 255."}
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
          error={!!errors.sd}
          helperText={
            errors.sd ||
            "Format: 6 hex digits, with or without 0x (e.g., 0x012030 or 012030). Leave empty to unset."
          }
          margin="normal"
          placeholder="0x012030"
          inputProps={{ spellCheck: false }}
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

export default EditOperatorSliceModal;
