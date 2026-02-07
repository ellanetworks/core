import React, { useCallback, useEffect, useState } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
  Alert,
  Collapse,
  FormControlLabel,
  Checkbox,
  Stack,
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { createAPIToken } from "@/queries/api_tokens";
import { useNavigate } from "react-router-dom";

import { LocalizationProvider } from "@mui/x-date-pickers/LocalizationProvider";
import { AdapterDayjs } from "@mui/x-date-pickers/AdapterDayjs";
import { DatePicker } from "@mui/x-date-pickers/DatePicker";
import dayjs, { Dayjs } from "dayjs";
import { useAuth } from "@/contexts/AuthContext";

interface CreateAPITokenModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: (token: string) => void;
}

const schema = yup.object({
  name: yup
    .string()
    .trim()
    .min(3, "Name must be at least 3 characters")
    .max(50, "Name must be at most 50 characters")
    .required("Name is required"),
  noExpiry: yup.boolean().required(),
  expiry: yup
    .mixed<Dayjs>()
    .nullable()
    .nullable()
    .test("expiry-required", "Expiry date is required", function (value) {
      const { noExpiry } = this.parent as { noExpiry: boolean };
      return noExpiry ? true : !!value;
    })
    .test(
      "expiry-future",
      "Expiry date must be in the future",
      function (value) {
        const { noExpiry } = this.parent as { noExpiry: boolean };
        if (noExpiry || !value) return true;
        return value.isAfter(dayjs().startOf("day"));
      },
    ),
});

const CreateAPITokenModal: React.FC<CreateAPITokenModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();
  if (!authReady || !accessToken) navigate("/login");

  const [formValues, setFormValues] = useState<{
    name: string;
    noExpiry: boolean;
    expiry: Dayjs | null;
  }>({
    name: "",
    noExpiry: false,
    expiry: null,
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const handleChange = <K extends keyof typeof formValues>(
    field: K,
    value: (typeof formValues)[K],
  ) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));
    validateField(field, value);
  };

  const handleBlur = (field: keyof typeof formValues) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const validateField = async (
    field: keyof typeof formValues,
    value: unknown,
  ) => {
    try {
      if (field === "expiry" || field === "noExpiry") {
        await schema.validateAt("expiry", {
          ...formValues,
          [field]: value,
        });
        setErrors((prev) => ({ ...prev, expiry: "" }));
      } else {
        const fieldSchema = yup.reach(schema, field) as yup.Schema<unknown>;
        await fieldSchema.validate(value);
        setErrors((prev) => ({ ...prev, [field]: "" }));
      }
    } catch (err) {
      if (err instanceof ValidationError) {
        const key =
          field === "expiry" || field === "noExpiry"
            ? "expiry"
            : (field as string);
        setErrors((prev) => ({ ...prev, [key]: err.message }));
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
            if (curr.path) acc[curr.path] = curr.message;
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
      // final validation before submit
      await schema.validate(formValues, { abortEarly: false });

      const expiryISO =
        formValues.noExpiry || !formValues.expiry
          ? ""
          : formValues.expiry.toDate().toISOString();

      const res = await createAPIToken(
        accessToken,
        formValues.name.trim(),
        expiryISO,
      );

      onClose();
      onSuccess(res.token);
    } catch (error: unknown) {
      let msg = "Unknown error occurred.";
      if (error instanceof ValidationError) {
        // collect field errors
        const validationErrors = error.inner.reduce(
          (acc, curr) => {
            if (curr.path) acc[curr.path] = curr.message;
            return acc;
          },
          {} as Record<string, string>,
        );
        setErrors(validationErrors);
        msg = "Please fix the errors above.";
      } else if (error instanceof Error) {
        msg = error.message;
      }

      setAlert({ message: `Failed to create API token: ${msg}` });
      console.error("Failed to create API token:", error);
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    onClose();
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      aria-labelledby="create-api-token-modal-title"
      aria-describedby="create-api-token-modal-description"
    >
      <DialogTitle id="create-api-token-modal-title">
        Create API Token
      </DialogTitle>

      <DialogContent id="create-api-token-modal-description" dividers>
        <Collapse in={!!alert.message}>
          <Alert
            onClose={() => setAlert({ message: "" })}
            sx={{ mb: 2 }}
            severity="error"
          >
            {alert.message}
          </Alert>
        </Collapse>

        <Stack spacing={2} sx={{ mt: 1, minWidth: { xs: 280, sm: 420 } }}>
          <TextField
            fullWidth
            label="Name"
            value={formValues.name}
            onChange={(e) => handleChange("name", e.target.value)}
            onBlur={() => handleBlur("name")}
            error={!!errors.name && touched.name}
            helperText={touched.name ? errors.name : "3–50 characters"}
            autoFocus
            margin="normal"
            placeholder="e.g., CI Pipeline, Local Script"
          />

          <LocalizationProvider dateAdapter={AdapterDayjs}>
            <DatePicker
              label="Expiry date"
              value={formValues.expiry}
              onChange={(val) => handleChange("expiry", val)}
              onClose={() => handleBlur("expiry")}
              disabled={formValues.noExpiry}
              minDate={dayjs().startOf("day")}
              slotProps={{
                textField: {
                  fullWidth: true,
                  error:
                    !!errors.expiry && touched.expiry && !formValues.noExpiry,
                  helperText:
                    touched.expiry && !formValues.noExpiry ? errors.expiry : "",
                },
              }}
            />
          </LocalizationProvider>

          <FormControlLabel
            control={
              <Checkbox
                checked={formValues.noExpiry}
                onChange={(e) => handleChange("noExpiry", e.target.checked)}
                onBlur={() => handleBlur("noExpiry")}
              />
            }
            label="No expiry"
          />
        </Stack>
      </DialogContent>

      <DialogActions>
        <Button onClick={handleClose}>Cancel</Button>
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

export default CreateAPITokenModal;
