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
import { useCookies } from "react-cookie";
import { listDataNetworks } from "@/queries/data_networks";

type DataNetwork = {
  name: string;
};

interface CreatePolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

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
  fiveQi: yup.number().min(0).max(256).required("5QI is required"),
  priorityLevel: yup
    .number()
    .min(0)
    .max(256)
    .required("Priority Level is required"),
  dataNetworkName: yup.string().required("Data Network Name is required."),
});

const CreatePolicyModal: React.FC<CreatePolicyModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const router = useRouter();
  const [cookies, ,] = useCookies(["user_token"]);

  if (!cookies.user_token) {
    router.push("/login");
  }

  const [formValues, setFormValues] = useState({
    name: "",
    bitrateUpValue: 100,
    bitrateUpUnit: "Mbps",
    bitrateDownValue: 100,
    bitrateDownUnit: "Mbps",
    fiveQi: 1,
    priorityLevel: 1,
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
      try {
        const dataNetworkDatae: DataNetwork[] = await listDataNetworks(
          cookies.user_token,
        );
        setDataNetworks(dataNetworkDatae.map((policy) => policy.name));
      } catch (error) {
        console.error("Failed to fetch data networks:", error);
      }
    };

    if (open) {
      fetchDataNetworks();
    }
  }, [open, cookies.user_token]);

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
    setLoading(true);
    setAlert({ message: "" });
    try {
      const bitrateUplink = `${formValues.bitrateUpValue} ${formValues.bitrateUpUnit}`;
      const bitrateDownlink = `${formValues.bitrateDownValue} ${formValues.bitrateDownUnit}`;
      await createPolicy(
        cookies.user_token,
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

      setAlert({
        message: `Failed to create policy: ${errorMessage}`,
      });
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
          <InputLabel id="demo-simple-select-label">
            Data Network Name
          </InputLabel>
          <Select
            value={formValues.dataNetworkName}
            onChange={(e) => handleChange("dataNetworkName", e.target.value)}
            onBlur={() => handleBlur("dataNetworkName")}
            error={!!errors.dataNetworkName && touched.dataNetworkName}
            labelId="demo-simple-select-label"
            label={"PolicyName"}
          >
            {dataNetworks.map((dataNetwork) => (
              <MenuItem key={dataNetwork} value={dataNetwork}>
                {dataNetwork}
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

export default CreatePolicyModal;
