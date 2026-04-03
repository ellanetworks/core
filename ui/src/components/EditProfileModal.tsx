import React, { useCallback, useState, useEffect } from "react";
import {
  Box,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
  Alert,
  Collapse,
  MenuItem,
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { APIProfile, updateProfile } from "@/queries/profiles";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface EditProfileModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APIProfile;
}

const schema = yup.object().shape({
  ambrUpValue: yup
    .number()
    .min(1, "Value must be between 1 and 999")
    .max(999, "Value must be between 1 and 999")
    .required("Value is required"),
  ambrUpUnit: yup.string().oneOf(["Mbps", "Gbps"], "Invalid unit"),
  ambrDownValue: yup
    .number()
    .min(1, "Value must be between 1 and 999")
    .max(999, "Value must be between 1 and 999")
    .required("Value is required"),
  ambrDownUnit: yup.string().oneOf(["Mbps", "Gbps"], "Invalid unit"),
});

function parseAmbr(value: string): { num: number; unit: string } {
  const parts = value.split(" ");
  if (parts.length === 2) {
    const num = Number(parts[0]);
    const unit = parts[1];
    if (!isNaN(num) && (unit === "Mbps" || unit === "Gbps")) {
      return { num, unit };
    }
  }
  return { num: 100, unit: "Mbps" };
}

const EditProfileModal: React.FC<EditProfileModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (open && authReady && !accessToken) {
      navigate("/login");
    }
  }, [open, authReady, accessToken, navigate]);

  const upParsed = parseAmbr(initialData.ue_ambr_uplink);
  const downParsed = parseAmbr(initialData.ue_ambr_downlink);

  const [formValues, setFormValues] = useState({
    ambrUpValue: upParsed.num,
    ambrUpUnit: upParsed.unit,
    ambrDownValue: downParsed.num,
    ambrDownUnit: downParsed.unit,
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    const up = parseAmbr(initialData.ue_ambr_uplink);
    const down = parseAmbr(initialData.ue_ambr_downlink);
    setFormValues({
      ambrUpValue: up.num,
      ambrUpUnit: up.unit,
      ambrDownValue: down.num,
      ambrDownUnit: down.unit,
    });
  }, [initialData]);

  const handleChange = (field: string, value: string | number) => {
    setFormValues((prev) => ({ ...prev, [field]: value }));
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
      const ueAmbrUplink = `${formValues.ambrUpValue} ${formValues.ambrUpUnit}`;
      const ueAmbrDownlink = `${formValues.ambrDownValue} ${formValues.ambrDownUnit}`;
      await updateProfile(
        accessToken,
        initialData.name,
        ueAmbrUplink,
        ueAmbrDownlink,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({ message: `Failed to update profile: ${errorMessage}` });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-profile-modal-title"
      fullWidth
      maxWidth="sm"
    >
      <DialogTitle id="edit-profile-modal-title">Edit Profile</DialogTitle>
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
          value={initialData.name}
          margin="normal"
          disabled
        />

        <Box display="flex" gap={2}>
          <TextField
            label="Bitrate Uplink"
            type="number"
            value={formValues.ambrUpValue}
            onChange={(e) =>
              handleChange("ambrUpValue", Number(e.target.value))
            }
            onBlur={() => handleBlur("ambrUpValue")}
            error={!!errors.ambrUpValue && touched.ambrUpValue}
            helperText={touched.ambrUpValue ? errors.ambrUpValue : ""}
            margin="normal"
          />
          <TextField
            select
            label="Unit"
            value={formValues.ambrUpUnit}
            onChange={(e) => handleChange("ambrUpUnit", e.target.value)}
            onBlur={() => handleBlur("ambrUpUnit")}
            margin="normal"
          >
            <MenuItem value="Mbps">Mbps</MenuItem>
            <MenuItem value="Gbps">Gbps</MenuItem>
          </TextField>
        </Box>

        <Box display="flex" gap={2}>
          <TextField
            label="Bitrate Downlink"
            type="number"
            value={formValues.ambrDownValue}
            onChange={(e) =>
              handleChange("ambrDownValue", Number(e.target.value))
            }
            onBlur={() => handleBlur("ambrDownValue")}
            error={!!errors.ambrDownValue && touched.ambrDownValue}
            helperText={touched.ambrDownValue ? errors.ambrDownValue : ""}
            margin="normal"
          />
          <TextField
            select
            label="Unit"
            value={formValues.ambrDownUnit}
            onChange={(e) => handleChange("ambrDownUnit", e.target.value)}
            onBlur={() => handleBlur("ambrDownUnit")}
            margin="normal"
          >
            <MenuItem value="Mbps">Mbps</MenuItem>
            <MenuItem value="Gbps">Gbps</MenuItem>
          </TextField>
        </Box>
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={!isValid || loading}
        >
          {loading ? "Saving..." : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default EditProfileModal;
