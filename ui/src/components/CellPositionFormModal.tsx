// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useCallback, useEffect, useState } from "react";
import {
  Alert,
  Button,
  Collapse,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  type SelectChangeEvent,
  TextField,
  Typography,
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import {
  createCellPosition,
  updateCellPosition,
  type CellPosition,
  type CellPositionRAT,
} from "@/queries/cell_positions";

interface CellPositionFormModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  /** When provided, the modal edits this record instead of creating a new one. */
  initial?: CellPosition | null;
  /** Pre-fills the gNB ID field, e.g. when opened from a specific radio's page. */
  defaultGnbId?: string;
  /** Pre-fills the RAT field, e.g. based on the originating radio's type. */
  defaultRat?: CellPositionRAT;
}

type FormValues = {
  rat: CellPositionRAT;
  mcc: string;
  mnc: string;
  cell_identity: string;
  gnb_id: string;
  latitude: string;
  longitude: string;
  altitude: string;
  uncertainty_semi_major: string;
  uncertainty_semi_minor: string;
  orientation_major: string;
  confidence: string;
};

// optionalNumber builds a yup test that accepts an empty string (field not
// provided) or a numeric string within the given bounds.
const optionalNumber = (label: string, min?: number, max?: number) =>
  yup.string().test(`${label}-optional-number`, (value, ctx) => {
    if (!value) return true;
    const n = Number(value);
    if (Number.isNaN(n)) {
      return ctx.createError({ message: `${label} must be a number` });
    }
    if (min !== undefined && n < min) {
      return ctx.createError({ message: `${label} must be at least ${min}` });
    }
    if (max !== undefined && n > max) {
      return ctx.createError({ message: `${label} must be at most ${max}` });
    }
    return true;
  });

const schema = yup.object().shape({
  rat: yup.string().oneOf(["nr", "eutra"]).required(),
  mcc: yup
    .string()
    .matches(/^\d{3}$/, "MCC must be 3 digits")
    .required("MCC is required"),
  mnc: yup
    .string()
    .matches(/^\d{2,3}$/, "MNC must be 2 or 3 digits")
    .required("MNC is required"),
  cell_identity: yup
    .string()
    .matches(/^[0-9a-fA-F]+$/, "Must be a hexadecimal string")
    .required("Cell Identity is required"),
  gnb_id: yup.string(),
  latitude: yup
    .string()
    .required("Latitude is required")
    .test("lat-range", "Must be between -90 and 90", (v) => {
      const n = Number(v);
      return !Number.isNaN(n) && n >= -90 && n <= 90;
    }),
  longitude: yup
    .string()
    .required("Longitude is required")
    .test("lon-range", "Must be between -180 and 180", (v) => {
      const n = Number(v);
      return !Number.isNaN(n) && n >= -180 && n <= 180;
    }),
  altitude: optionalNumber("Altitude"),
  uncertainty_semi_major: optionalNumber("Uncertainty (semi-major)", 0),
  uncertainty_semi_minor: optionalNumber("Uncertainty (semi-minor)", 0),
  orientation_major: optionalNumber("Orientation", 0, 359),
  confidence: optionalNumber("Confidence", 0, 100),
});

const emptyValues = (
  defaultGnbId?: string,
  defaultRat?: CellPositionRAT,
): FormValues => ({
  rat: defaultRat ?? "nr",
  mcc: "",
  mnc: "",
  cell_identity: "",
  gnb_id: defaultGnbId ?? "",
  latitude: "",
  longitude: "",
  altitude: "",
  uncertainty_semi_major: "",
  uncertainty_semi_minor: "",
  orientation_major: "",
  confidence: "",
});

const valuesFromExisting = (cp: CellPosition): FormValues => ({
  rat: cp.rat,
  mcc: cp.mcc,
  mnc: cp.mnc,
  cell_identity: cp.cell_identity,
  gnb_id: cp.gnb_id ?? "",
  latitude: String(cp.latitude),
  longitude: String(cp.longitude),
  altitude: cp.altitude != null ? String(cp.altitude) : "",
  uncertainty_semi_major:
    cp.uncertainty_semi_major != null ? String(cp.uncertainty_semi_major) : "",
  uncertainty_semi_minor:
    cp.uncertainty_semi_minor != null ? String(cp.uncertainty_semi_minor) : "",
  orientation_major:
    cp.orientation_major != null ? String(cp.orientation_major) : "",
  confidence: cp.confidence != null ? String(cp.confidence) : "",
});

const CellPositionFormModal: React.FC<CellPositionFormModalProps> = ({
  open,
  onClose,
  onSuccess,
  initial,
  defaultGnbId,
  defaultRat,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();
  const isEdit = !!initial;

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState<FormValues>(() =>
    initial
      ? valuesFromExisting(initial)
      : emptyValues(defaultGnbId, defaultRat),
  );

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const handleChange = (field: keyof FormValues, value: string) => {
    setFormValues((prev) => ({ ...prev, [field]: value }));
    validateField(field, value);
  };

  const handleBlur = (field: keyof FormValues) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const validateField = async (field: keyof FormValues, value: string) => {
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

    const params = {
      rat: formValues.rat,
      mcc: formValues.mcc.trim(),
      mnc: formValues.mnc.trim(),
      cell_identity: formValues.cell_identity.trim(),
      gnb_id: formValues.gnb_id.trim() || undefined,
      latitude: Number(formValues.latitude),
      longitude: Number(formValues.longitude),
      altitude: formValues.altitude ? Number(formValues.altitude) : undefined,
      uncertainty_semi_major: formValues.uncertainty_semi_major
        ? Number(formValues.uncertainty_semi_major)
        : undefined,
      uncertainty_semi_minor: formValues.uncertainty_semi_minor
        ? Number(formValues.uncertainty_semi_minor)
        : undefined,
      orientation_major: formValues.orientation_major
        ? Number(formValues.orientation_major)
        : undefined,
      confidence: formValues.confidence
        ? Number(formValues.confidence)
        : undefined,
    };

    try {
      if (isEdit && initial) {
        await updateCellPosition(accessToken, initial.id, params);
      } else {
        await createCellPosition(accessToken, params);
      }
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const message =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to ${isEdit ? "update" : "create"} cell position: ${message}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="cell-position-modal-title"
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle id="cell-position-modal-title">
        {isEdit ? "Edit Cell Position" : "Add Cell Position"}
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

        <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
          Identifies the serving cell (RAT + PLMN + cell identity) and its
          antenna coordinates, used to anchor Cell-ID / E-CID location
          estimates.
        </Typography>

        <FormControl fullWidth margin="normal">
          <InputLabel id="cell-position-rat-label">RAT</InputLabel>
          <Select
            labelId="cell-position-rat-label"
            label="RAT"
            value={formValues.rat}
            onChange={(e: SelectChangeEvent) =>
              handleChange("rat", e.target.value as CellPositionRAT)
            }
          >
            <MenuItem value="nr">NR (5G)</MenuItem>
            <MenuItem value="eutra">E-UTRA (4G)</MenuItem>
          </Select>
        </FormControl>

        <TextField
          fullWidth
          label="MCC"
          value={formValues.mcc}
          onChange={(e) => handleChange("mcc", e.target.value)}
          onBlur={() => handleBlur("mcc")}
          error={!!errors.mcc && touched.mcc}
          helperText={touched.mcc ? errors.mcc : "e.g. 001"}
          margin="normal"
          autoFocus
        />
        <TextField
          fullWidth
          label="MNC"
          value={formValues.mnc}
          onChange={(e) => handleChange("mnc", e.target.value)}
          onBlur={() => handleBlur("mnc")}
          error={!!errors.mnc && touched.mnc}
          helperText={touched.mnc ? errors.mnc : "e.g. 01"}
          margin="normal"
        />
        <TextField
          fullWidth
          label="Cell Identity"
          value={formValues.cell_identity}
          onChange={(e) => handleChange("cell_identity", e.target.value)}
          onBlur={() => handleBlur("cell_identity")}
          error={!!errors.cell_identity && touched.cell_identity}
          helperText={
            touched.cell_identity
              ? errors.cell_identity
              : "Hex NR Cell Identity (36-bit) or E-UTRA Cell Identity (28-bit), e.g. 00066c000"
          }
          margin="normal"
        />
        <TextField
          fullWidth
          label="gNB ID"
          value={formValues.gnb_id}
          onChange={(e) => handleChange("gnb_id", e.target.value)}
          onBlur={() => handleBlur("gnb_id")}
          helperText="Optional. Matches the Radio ID shown on the Radios page, used to group cells under a radio."
          margin="normal"
        />
        <TextField
          fullWidth
          label="Latitude"
          value={formValues.latitude}
          onChange={(e) => handleChange("latitude", e.target.value)}
          onBlur={() => handleBlur("latitude")}
          error={!!errors.latitude && touched.latitude}
          helperText={
            touched.latitude ? errors.latitude : "WGS-84 decimal degrees"
          }
          margin="normal"
        />
        <TextField
          fullWidth
          label="Longitude"
          value={formValues.longitude}
          onChange={(e) => handleChange("longitude", e.target.value)}
          onBlur={() => handleBlur("longitude")}
          error={!!errors.longitude && touched.longitude}
          helperText={
            touched.longitude ? errors.longitude : "WGS-84 decimal degrees"
          }
          margin="normal"
        />
        <TextField
          fullWidth
          label="Altitude"
          value={formValues.altitude}
          onChange={(e) => handleChange("altitude", e.target.value)}
          onBlur={() => handleBlur("altitude")}
          error={!!errors.altitude && touched.altitude}
          helperText={touched.altitude ? errors.altitude : "Optional, metres"}
          margin="normal"
        />
        <TextField
          fullWidth
          label="Uncertainty (semi-major)"
          value={formValues.uncertainty_semi_major}
          onChange={(e) =>
            handleChange("uncertainty_semi_major", e.target.value)
          }
          onBlur={() => handleBlur("uncertainty_semi_major")}
          error={
            !!errors.uncertainty_semi_major && touched.uncertainty_semi_major
          }
          helperText={
            touched.uncertainty_semi_major
              ? errors.uncertainty_semi_major
              : "Optional, metres"
          }
          margin="normal"
        />
        <TextField
          fullWidth
          label="Uncertainty (semi-minor)"
          value={formValues.uncertainty_semi_minor}
          onChange={(e) =>
            handleChange("uncertainty_semi_minor", e.target.value)
          }
          onBlur={() => handleBlur("uncertainty_semi_minor")}
          error={
            !!errors.uncertainty_semi_minor && touched.uncertainty_semi_minor
          }
          helperText={
            touched.uncertainty_semi_minor
              ? errors.uncertainty_semi_minor
              : "Optional, metres"
          }
          margin="normal"
        />
        <TextField
          fullWidth
          label="Orientation (major axis)"
          value={formValues.orientation_major}
          onChange={(e) => handleChange("orientation_major", e.target.value)}
          onBlur={() => handleBlur("orientation_major")}
          error={!!errors.orientation_major && touched.orientation_major}
          helperText={
            touched.orientation_major
              ? errors.orientation_major
              : "Optional, degrees (0–359)"
          }
          margin="normal"
        />
        <TextField
          fullWidth
          label="Confidence"
          value={formValues.confidence}
          onChange={(e) => handleChange("confidence", e.target.value)}
          onBlur={() => handleBlur("confidence")}
          error={!!errors.confidence && touched.confidence}
          helperText={
            touched.confidence ? errors.confidence : "Optional, percent (0–100)"
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
          {loading ? "Saving..." : isEdit ? "Save" : "Create"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default CellPositionFormModal;
