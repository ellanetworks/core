import React, { useCallback, useState, useEffect } from "react";
import {
  Box,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  FormControl,
  Select,
  InputLabel,
  Button,
  MenuItem,
  Alert,
  Collapse,
} from "@mui/material";
import {
  updatePolicy,
  getPolicy,
  type APIPolicy,
  type PolicyRules,
} from "@/queries/policies";
import {
  listDataNetworks,
  type ListDataNetworksResponse,
} from "@/queries/data_networks";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import * as yup from "yup";
import { ValidationError } from "yup";

interface EditPolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APIPolicy;
}

type FormState = {
  name: string;
  bitrateUpValue: number;
  bitrateUpUnit: "Mbps" | "Gbps";
  bitrateDownValue: number;
  bitrateDownUnit: "Mbps" | "Gbps";
  var5qi: number;
  arp: number;
  data_network_name: string;
};

const PER_PAGE = 12;

const NON_GBR_5QI_OPTIONS: { value: number; label: string }[] = [
  { value: 5, label: "5 — IMS Signalling" },
  { value: 6, label: "6 — TCP (buffered streaming, web)" },
  { value: 7, label: "7 — Voice, live video, gaming" },
  { value: 8, label: "8 — TCP (buffered streaming)" },
  { value: 9, label: "9 — TCP (default)" },
  { value: 69, label: "69 — Mission critical signalling" },
  { value: 70, label: "70 — Mission critical data" },
  { value: 79, label: "79 — V2X messages" },
  { value: 80, label: "80 — Low latency eMBB" },
];

const NON_GBR_5QI_VALUES = NON_GBR_5QI_OPTIONS.map((o) => o.value);

const policySchema = yup.object().shape({
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
  var5qi: yup
    .number()
    .oneOf(
      NON_GBR_5QI_VALUES,
      `5QI must be one of: ${NON_GBR_5QI_VALUES.join(", ")}`,
    )
    .required("5QI is required"),
  arp: yup.number().min(1).max(15).required("ARP is required"),
  data_network_name: yup.string().required("Data Network Name is required."),
});

const EditPolicyModal: React.FC<EditPolicyModalProps> = ({
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

  const [formValues, setFormValues] = useState<FormState>({
    name: "",
    bitrateUpValue: 0,
    bitrateUpUnit: "Mbps",
    bitrateDownValue: 0,
    bitrateDownUnit: "Mbps",
    var5qi: 0,
    arp: 0,
    data_network_name: "",
  });

  const [dataNetworks, setDataNetworks] = useState<string[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  // Store current rules so we can preserve them on submit
  const [currentRules, setCurrentRules] = useState<PolicyRules | undefined>(
    undefined,
  );

  // Fetch full policy data (including rules) when modal opens
  useEffect(() => {
    if (!open || !accessToken) return;

    const fetchFullPolicy = async () => {
      try {
        const fullPolicy = await getPolicy(accessToken, initialData.name);

        const [bitrateUpValueStr, bitrateUpUnit] =
          fullPolicy.bitrate_uplink.split(" ");
        const [bitrateDownValueStr, bitrateDownUnit] =
          fullPolicy.bitrate_downlink.split(" ");

        setFormValues({
          name: fullPolicy.name,
          bitrateUpValue: parseInt(bitrateUpValueStr, 10),
          bitrateUpUnit: (bitrateUpUnit as "Mbps" | "Gbps") ?? "Mbps",
          bitrateDownValue: parseInt(bitrateDownValueStr, 10),
          bitrateDownUnit: (bitrateDownUnit as "Mbps" | "Gbps") ?? "Mbps",
          var5qi: fullPolicy.var5qi,
          arp: fullPolicy.arp,
          data_network_name: fullPolicy.data_network_name,
        });
        setErrors({});
        setTouched({});

        // Preserve existing rules
        setCurrentRules(fullPolicy.rules);
      } catch (error) {
        console.error("Failed to fetch policy data:", error);
      }
    };

    fetchFullPolicy();
  }, [open, initialData.name, accessToken]);

  // Fetch data networks
  useEffect(() => {
    const fetchDataNetworks = async () => {
      if (!open || !accessToken) return;
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

  const handleChange = (field: keyof FormState, value: string | number) => {
    setFormValues((prev) => ({ ...prev, [field]: value as never }));
    validateField(field, value);
  };

  const handleBlur = (field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const validateField = async (field: string, value: string | number) => {
    try {
      await policySchema.validateAt(field, { ...formValues, [field]: value });
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
      await policySchema.validate(formValues, { abortEarly: false });
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

  const fiveQiIsAllowed = NON_GBR_5QI_VALUES.includes(formValues.var5qi);

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });

    const bitrateUp = `${formValues.bitrateUpValue} ${formValues.bitrateUpUnit}`;
    const bitrateDown = `${formValues.bitrateDownValue} ${formValues.bitrateDownUnit}`;

    try {
      await updatePolicy(
        accessToken,
        formValues.name,
        bitrateUp,
        bitrateDown,
        formValues.var5qi,
        formValues.arp,
        formValues.data_network_name,
        currentRules,
      );

      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({ message: `Failed to update policy: ${errorMessage}` });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-policy-modal-title"
      aria-describedby="edit-policy-modal-description"
      fullWidth
      maxWidth="sm"
    >
      <DialogTitle id="edit-policy-modal-title">Edit Policy</DialogTitle>
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
          margin="normal"
          disabled
        />

        <FormControl fullWidth margin="normal">
          <InputLabel id="data-network-select-label">
            Data Network Name
          </InputLabel>
          <Select
            labelId="data-network-select-label"
            autoFocus
            label="Data Network Name"
            value={formValues.data_network_name}
            onChange={(e) => handleChange("data_network_name", e.target.value)}
            onBlur={() => handleBlur("data_network_name")}
            error={!!errors.data_network_name && touched.data_network_name}
          >
            {dataNetworks.map((name) => (
              <MenuItem key={name} value={name}>
                {name}
              </MenuItem>
            ))}
          </Select>
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
            margin="normal"
          >
            <MenuItem value="Mbps">Mbps</MenuItem>
            <MenuItem value="Gbps">Gbps</MenuItem>
          </TextField>
        </Box>

        <FormControl fullWidth margin="normal">
          <InputLabel id="fiveqi-edit-select-label">5QI (non-GBR)</InputLabel>
          <Select
            labelId="fiveqi-edit-select-label"
            label="5QI (non-GBR)"
            value={formValues.var5qi}
            onChange={(e) => handleChange("var5qi", Number(e.target.value))}
            onBlur={() => handleBlur("var5qi")}
            error={!!errors.var5qi && touched.var5qi}
          >
            {!fiveQiIsAllowed && (
              <MenuItem value={formValues.var5qi} disabled>
                {formValues.var5qi} (current, unsupported)
              </MenuItem>
            )}
            {NON_GBR_5QI_OPTIONS.map((opt) => (
              <MenuItem key={opt.value} value={opt.value}>
                {opt.label}
              </MenuItem>
            ))}
          </Select>
        </FormControl>

        <TextField
          fullWidth
          label="Allocation and Retention Priority (ARP)"
          type="number"
          value={formValues.arp}
          onChange={(e) => handleChange("arp", Number(e.target.value))}
          onBlur={() => handleBlur("arp")}
          error={!!errors.arp && touched.arp}
          helperText={
            touched.arp && errors.arp
              ? errors.arp
              : "1 (highest) to 15 (lowest)"
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
          {loading ? "Updating..." : "Update"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default EditPolicyModal;
