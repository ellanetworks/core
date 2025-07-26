"use client";
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
  MenuItem,
  FormControlLabel,
  Checkbox,
} from "@mui/material";
import * as yup from "yup";
import { isSchema } from "yup";
import { ValidationError } from "yup";
import { createRoute } from "@/queries/routes";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

const cidrRegex =
  /^((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)\.){3}(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)\/([1-9]|[1-2]\d|3[0-2])$/;
const ipv4Regex =
  /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;

const schema = yup.object().shape({
  defaultRoute: yup.boolean(),
  destination: yup.string().when(["defaultRoute"], (values, schema) => {
    const defaultRoute = values[0] as boolean;
    if (defaultRoute) {
      return schema.oneOf(
        ["0.0.0.0/0"],
        "For a default route, destination must be 0.0.0.0/0",
      );
    } else {
      return schema
        .required("Destination is required")
        .matches(cidrRegex, "Destination must be a valid CIDR (IPv4)");
    }
  }),
  gateway: yup
    .string()
    .required("Gateway is required")
    .matches(ipv4Regex, "Gateway must be a valid IPv4 address"),
  interface: yup
    .string()
    .oneOf(["n3", "n6"], "Interface must be either n3 or n6")
    .required("Interface is required"),
  metric: yup.number().required("Metric is required"),
});

interface CreateRouteModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

type FormValues = {
  defaultRoute: boolean;
  destination: string;
  gateway: string;
  interface: string;
  metric: number;
};

const CreateRouteModal: React.FC<CreateRouteModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const router = useRouter();
  const [cookies] = useCookies(["user_token"]);

  if (!cookies.user_token) {
    router.push("/login");
  }

  // Set default interface to "n6" and defaultRoute to false.
  const [formValues, setFormValues] = useState<FormValues>({
    destination: "",
    gateway: "",
    interface: "n6",
    metric: 0,
    defaultRoute: false,
  });

  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const handleChange = (field: string, value: string | number | boolean) => {
    // When toggling defaultRoute, update the destination accordingly.
    if (field === "defaultRoute" && typeof value === "boolean") {
      setFormValues((prev) => ({
        ...prev,
        defaultRoute: value,
        destination: value ? "0.0.0.0/0" : "",
      }));
      validateField("destination", value ? "0.0.0.0/0" : "");
    } else {
      setFormValues((prev) => ({
        ...prev,
        [field]: value,
      }));
      validateField(field, value);
    }
  };

  const handleBlur = (field: string) => {
    setTouched((prev) => ({
      ...prev,
      [field]: true,
    }));
  };

  const validateField = async (
    field: string,
    value: string | number | boolean,
  ) => {
    try {
      const fieldSchema = yup.reach(schema, field);
      if (!isSchema(fieldSchema)) {
        throw new Error(`Field "${field}" does not resolve to a schema`);
      }

      await fieldSchema.validate(value);
      setErrors((prev) => ({
        ...prev,
        [field]: "",
      }));
    } catch (err) {
      if (err instanceof ValidationError) {
        setErrors((prev) => ({
          ...prev,
          [field]: err.message,
        }));
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
    setLoading(true);
    setAlert({ message: "" });
    try {
      await createRoute(
        cookies.user_token,
        formValues.destination,
        formValues.gateway,
        formValues.interface,
        formValues.metric,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to create route: ${errorMessage}`,
      });
      console.error("Failed to create route:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="create-route-modal-title"
      aria-describedby="create-route-modal-description"
    >
      <DialogTitle id="create-route-modal-title">Create Route</DialogTitle>
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
        <FormControlLabel
          control={
            <Checkbox
              checked={formValues.defaultRoute}
              onChange={(e) => handleChange("defaultRoute", e.target.checked)}
            />
          }
          label="Default Route (0.0.0.0/0)"
        />
        <TextField
          fullWidth
          label="Destination"
          value={formValues.destination}
          onChange={(e) => handleChange("destination", e.target.value)}
          onBlur={() => handleBlur("destination")}
          error={!!errors.destination && touched.destination}
          helperText={touched.destination ? errors.destination : ""}
          margin="normal"
          disabled={formValues.defaultRoute}
        />
        <TextField
          fullWidth
          label="Gateway"
          value={formValues.gateway}
          onChange={(e) => handleChange("gateway", e.target.value)}
          onBlur={() => handleBlur("gateway")}
          error={!!errors.gateway && touched.gateway}
          helperText={touched.gateway ? errors.gateway : ""}
          margin="normal"
        />
        <TextField
          fullWidth
          select
          label="Interface"
          value={formValues.interface}
          onChange={(e) => handleChange("interface", e.target.value)}
          onBlur={() => handleBlur("interface")}
          error={!!errors.interface && touched.interface}
          helperText={touched.interface ? errors.interface : ""}
          margin="normal"
        >
          <MenuItem value="n3">n3</MenuItem>
          <MenuItem value="n6">n6</MenuItem>
        </TextField>
        <TextField
          fullWidth
          label="Metric"
          type="number"
          value={formValues.metric}
          onChange={(e) => handleChange("metric", Number(e.target.value))}
          onBlur={() => handleBlur("metric")}
          error={!!errors.metric && touched.metric}
          helperText={touched.metric ? errors.metric : ""}
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

export default CreateRouteModal;
