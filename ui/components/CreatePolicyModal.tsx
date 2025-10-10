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
  FormControl,
  Typography,
  InputLabel,
  Select,
  Collapse,
  MenuItem,
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { createPolicy } from "@/queries/policies";
import { useRouter } from "next/navigation";
import {
  listDataNetworks,
  type ListDataNetworksResponse,
} from "@/queries/data_networks";
import { useAuth } from "@/contexts/AuthContext";

interface CreatePolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const NON_GBR_5QI_OPTIONS = [5, 6, 7, 8, 9, 69, 70, 79, 80];

const schema = yup.object().shape({
  name: yup.string().min(1).max(256).required("Name is required"),
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
  fiveQi: yup
    .number()
    .oneOf(
      NON_GBR_5QI_OPTIONS,
      `5QI must be one of: ${NON_GBR_5QI_OPTIONS.join(", ")}`,
    )
    .required("5QI is required"),
  priorityLevel: yup
    .number()
    .min(0)
    .max(256)
    .required("Priority Level is required"),
  dataNetworkName: yup.string().required("Data Network Name is required."),
});

const PER_PAGE = 12; // fetch up to 12 DNs for the dropdown

const CreatePolicyModal: React.FC<CreatePolicyModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const router = useRouter();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (open && authReady && !accessToken) {
      router.push("/login");
    }
  }, [open, authReady, accessToken, router]);

  const [formValues, setFormValues] = useState({
    name: "",
    bitrateUpValue: 100,
    bitrateUpUnit: "Mbps",
    bitrateDownValue: 100,
    bitrateDownUnit: "Mbps",
    fiveQi: 9,
    priorityLevel: 90,
    dataNetworkName: "",
  });

  const [dataNetworks, setDataNetworks] = useState<string[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    const fetchDataNetworks = async () => {
      if (!accessToken || !open) return;
      try {
        const res: ListDataNetworksResponse = await listDataNetworks(
          accessToken,
          1,
          PER_PAGE,
        );
        setDataNetworks((res.items ?? []).map((dn) => dn.name));
      } catch (error) {
        console.error("Failed to fetch data networks:", error);
      }
    };
    fetchDataNetworks();
  }, [open, accessToken]);

  const handleChange = (field: string, value: string | number) => {
    setFormValues((prev) => ({ ...prev, [field]: value }));
    validateField(field, value);
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
      const bitrateUplink = `${formValues.bitrateUpValue} ${formValues.bitrateUpUnit}`;
      const bitrateDownlink = `${formValues.bitrateDownValue} ${formValues.bitrateDownUnit}`;
      await createPolicy(
        accessToken,
        formValues.name,
        bitrateUplink,
        bitrateDownlink,
        formValues.fiveQi,
        formValues.priorityLevel,
        formValues.dataNetworkName,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      let errorMessage = "Unknown error occurred.";
      if (error instanceof Error) {
        errorMessage = error.message;
      }
      setAlert({ message: `Failed to create policy: ${errorMessage}` });
      console.error("Failed to create policy:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="create-policy-modal-title"
      aria-describedby="create-policy-modal-description"
    >
      <DialogTitle>Create Policy</DialogTitle>
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

        <FormControl fullWidth margin="normal">
          <InputLabel id="data-network-select-label">
            Data Network Name
          </InputLabel>
          <Select
            labelId="data-network-select-label"
            label="Data Network Name"
            value={formValues.dataNetworkName}
            onChange={(e) => handleChange("dataNetworkName", e.target.value)}
            onBlur={() => handleBlur("dataNetworkName")}
            error={!!errors.dataNetworkName && touched.dataNetworkName}
          >
            {dataNetworks.map((name) => (
              <MenuItem key={name} value={name}>
                {name}
              </MenuItem>
            ))}
          </Select>
          {touched.dataNetworkName && errors.dataNetworkName && (
            <Typography color="error" variant="caption">
              {errors.dataNetworkName}
            </Typography>
          )}
        </FormControl>

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

        <FormControl fullWidth margin="normal">
          <InputLabel id="fiveqi-select-label">5QI (non-GBR)</InputLabel>
          <Select
            labelId="fiveqi-select-label"
            label="5QI (non-GBR)"
            value={formValues.fiveQi}
            onChange={(e) => handleChange("fiveQi", Number(e.target.value))}
            onBlur={() => handleBlur("fiveQi")}
            error={!!errors.fiveQi && touched.fiveQi}
          >
            {NON_GBR_5QI_OPTIONS.map((val) => (
              <MenuItem key={val} value={val}>
                {val}
              </MenuItem>
            ))}
          </Select>
          {touched.fiveQi && errors.fiveQi && (
            <Typography color="error" variant="caption">
              {errors.fiveQi}
            </Typography>
          )}
        </FormControl>

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

export default CreatePolicyModal;
