import React, { useState, useEffect } from "react";
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
import { createProfile } from "@/queries/profiles";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface CreateProfileModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object().shape({
  name: yup.string().min(1).max(256).required("Name is required"),
  ipPool: yup
    .string()
    .matches(
      /^(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\/\d{1,2}$/,
      "Must be a valid IP pool (e.g., 10.45.0.0/16)",
    )
    .required("IP Pool is required"),
  dns: yup
    .string()
    .matches(
      /^(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)$/,
      "Must be a valid IP address",
    )
    .required("DNS is required"),
  mtu: yup.number().min(1).max(65535).required("MTU is required"),
  bitrateUpValue: yup
    .number()
    .min(1, "Bitrate value must be between 1 and 999")
    .max(999, "Bitrate value must be between 1 and 999")
    .required("Bitrate value is required"),
  bitrateUpUnit: yup.string().oneOf(["Mbps", "Gbps"], "Invalid unit"),
  bitrateDownValue: yup
    .number()
    .min(1, "Bitrate value must be between 1 and 999")
    .max(999, "Bitrate value must be between 1 and 999")
    .required("Bitrate value is required"),
  bitrateDownUnit: yup.string().oneOf(["Mbps", "Gbps"], "Invalid unit"),
  fiveQi: yup.number().min(0).max(256).required("5QI is required"),
  priorityLevel: yup
    .number()
    .min(0)
    .max(256)
    .required("Priority Level is required"),
});

const CreateProfileModal: React.FC<CreateProfileModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const router = useRouter();
  const [cookies, setCookie, removeCookie] = useCookies(["user_token"]);

  if (!cookies.user_token) {
    router.push("/login");
  }

  const [formValues, setFormValues] = useState({
    name: "",
    ipPool: "10.45.0.0/16",
    dns: "8.8.8.8",
    mtu: 1500,
    bitrateUpValue: 100,
    bitrateUpUnit: "Mbps",
    bitrateDownValue: 100,
    bitrateDownUnit: "Mbps",
    fiveQi: 1,
    priorityLevel: 1,
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
      const fieldSchema = yup.reach(schema, field) as yup.Schema<unknown>;
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

  const validateForm = async () => {
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
  };

  useEffect(() => {
    validateForm();
  }, [formValues]);

  const handleSubmit = async () => {
    setLoading(true);
    setAlert({ message: "" });
    try {
      const bitrateUplink = `${formValues.bitrateUpValue} ${formValues.bitrateUpUnit}`;
      const bitrateDownlink = `${formValues.bitrateDownValue} ${formValues.bitrateDownUnit}`;
      await createProfile(
        cookies.user_token,
        formValues.name,
        formValues.ipPool,
        formValues.dns,
        formValues.mtu,
        bitrateUplink,
        bitrateDownlink,
        formValues.fiveQi,
        formValues.priorityLevel,
      );
      onClose();
      onSuccess();
    } catch (error: any) {
      const errorMessage = error?.message || "Unknown error occurred.";
      setAlert({
        message: `Failed to create profile: ${errorMessage}`,
      });
      console.error("Failed to create profile:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="create-profile-modal-title"
      aria-describedby="create-profile-modal-description"
    >
      <DialogTitle>Create Profile</DialogTitle>
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
        />
        <TextField
          fullWidth
          label="IP Pool"
          value={formValues.ipPool}
          onChange={(e) => handleChange("ipPool", e.target.value)}
          onBlur={() => handleBlur("ipPool")}
          error={!!errors.ipPool && touched.ipPool}
          helperText={touched.ipPool ? errors.ipPool : ""}
          margin="normal"
        />
        <TextField
          fullWidth
          label="DNS"
          value={formValues.dns}
          onChange={(e) => handleChange("dns", e.target.value)}
          onBlur={() => handleBlur("dns")}
          error={!!errors.dns && touched.dns}
          helperText={touched.dns ? errors.dns : ""}
          margin="normal"
        />
        <TextField
          fullWidth
          label="MTU"
          type="number"
          value={formValues.mtu}
          onChange={(e) => handleChange("mtu", Number(e.target.value))}
          onBlur={() => handleBlur("mtu")}
          error={!!errors.mtu && touched.mtu}
          helperText={touched.mtu ? errors.mtu : ""}
          margin="normal"
        />
        <Box display="flex" gap={2}>
          <TextField
            label="Bitrate Up Value"
            type="number"
            value={formValues.bitrateUpValue}
            onChange={(e) =>
              handleChange("bitrateUpValue", Number(e.target.value))
            }
            onBlur={() => handleBlur("bitrateUpValue")}
            error={!!errors.bitrateUpValue && touched.bitrateUpValue}
            helperText={touched.bitrateUpValue ? errors.bitrateUpValue : ""}
            margin="normal"
          />
          <TextField
            select
            label="Unit"
            value={formValues.bitrateUpUnit}
            onChange={(e) => handleChange("bitrateUpUnit", e.target.value)}
            onBlur={() => handleBlur("bitrateUpUnit")}
            error={!!errors.bitrateUpUnit && touched.bitrateUpUnit}
            helperText={touched.bitrateUpUnit ? errors.bitrateUpUnit : ""}
            margin="normal"
          >
            <MenuItem value="Mbps">Mbps</MenuItem>
            <MenuItem value="Gbps">Gbps</MenuItem>
          </TextField>
        </Box>
        <Box display="flex" gap={2}>
          <TextField
            label="Bitrate Down Value"
            type="number"
            value={formValues.bitrateDownValue}
            onChange={(e) =>
              handleChange("bitrateDownValue", Number(e.target.value))
            }
            onBlur={() => handleBlur("bitrateDownValue")}
            error={!!errors.bitrateDownValue && touched.bitrateDownValue}
            helperText={touched.bitrateDownValue ? errors.bitrateDownValue : ""}
            margin="normal"
          />
          <TextField
            select
            label="Unit"
            value={formValues.bitrateDownUnit}
            onChange={(e) => handleChange("bitrateDownUnit", e.target.value)}
            onBlur={() => handleBlur("bitrateDownUnit")}
            error={!!errors.bitrateDownUnit && touched.bitrateDownUnit}
            helperText={touched.bitrateDownUnit ? errors.bitrateDownUnit : ""}
            margin="normal"
          >
            <MenuItem value="Mbps">Mbps</MenuItem>
            <MenuItem value="Gbps">Gbps</MenuItem>
          </TextField>
        </Box>
        <TextField
          fullWidth
          label="5QI"
          type="number"
          value={formValues.fiveQi}
          onChange={(e) => handleChange("fiveQi", Number(e.target.value))}
          onBlur={() => handleBlur("fiveQi")}
          error={!!errors.fiveQi && touched.fiveQi}
          helperText={touched.fiveQi ? errors.fiveQi : ""}
          margin="normal"
        />
        <TextField
          fullWidth
          label="Priority Level"
          type="number"
          value={formValues.priorityLevel}
          onChange={(e) =>
            handleChange("priorityLevel", Number(e.target.value))
          }
          onBlur={() => handleBlur("priorityLevel")}
          error={!!errors.priorityLevel && touched.priorityLevel}
          helperText={touched.priorityLevel ? errors.priorityLevel : ""}
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

export default CreateProfileModal;
