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
import { useNavigate } from "react-router-dom";
import {
  listDataNetworks,
  type ListDataNetworksResponse,
} from "@/queries/data_networks";
import { listSlices, type ListSlicesResponse } from "@/queries/slices";
import { useAuth } from "@/contexts/AuthContext";

interface CreatePolicyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  profileName: string;
}

const NON_GBR_5QI_OPTIONS: { value: number; label: string }[] = [
  { value: 5, label: "5 — IMS signalling" },
  { value: 6, label: "6 — Buffered streaming, web browsing" },
  { value: 7, label: "7 — Voice, live video, interactive gaming" },
  { value: 8, label: "8 — Buffered streaming" },
  { value: 9, label: "9 — Best effort (default)" },
  { value: 69, label: "69 — Mission critical signalling" },
  { value: 70, label: "70 — Mission critical data" },
  { value: 79, label: "79 — V2X messages" },
  { value: 80, label: "80 — Low latency eMBB" },
];

const NON_GBR_5QI_VALUES = NON_GBR_5QI_OPTIONS.map((o) => o.value);

const schema = yup.object().shape({
  name: yup.string().min(1).max(256).required("Name is required"),
  sliceName: yup.string().required("Slice is required."),
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
  fiveQi: yup
    .number()
    .oneOf(
      NON_GBR_5QI_VALUES,
      `5QI must be one of: ${NON_GBR_5QI_VALUES.join(", ")}`,
    )
    .required("5QI is required"),
  arp: yup.number().min(1).max(15).required("ARP is required"),
  dataNetworkName: yup.string().required("Data Network is required."),
});

const PER_PAGE = 12; // fetch up to 12 items for dropdowns

const CreatePolicyModal: React.FC<CreatePolicyModalProps> = ({
  open,
  onClose,
  onSuccess,
  profileName,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (open && authReady && !accessToken) {
      navigate("/login");
    }
  }, [open, authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState({
    name: "",
    sliceName: "",
    ambrUpValue: 100,
    ambrUpUnit: "Mbps",
    ambrDownValue: 100,
    ambrDownUnit: "Mbps",
    fiveQi: 9,
    arp: 1,
    dataNetworkName: "",
  });

  const [dataNetworks, setDataNetworks] = useState<string[]>([]);
  const [slices, setSlices] = useState<string[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    const fetchDropdownData = async () => {
      if (!accessToken || !open) return;
      try {
        const [dnRes, sliceRes] = await Promise.all([
          listDataNetworks(
            accessToken,
            1,
            PER_PAGE,
          ) as Promise<ListDataNetworksResponse>,
          listSlices(accessToken, 1, PER_PAGE) as Promise<ListSlicesResponse>,
        ]);
        setDataNetworks((dnRes.items ?? []).map((dn) => dn.name));
        setSlices((sliceRes.items ?? []).map((s) => s.name));
      } catch (error) {
        console.error("Failed to fetch dropdown data:", error);
        setAlert({
          message: "Failed to load dropdown data. Please close and try again.",
        });
      }
    };
    fetchDropdownData();
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

  useEffect(() => {
    if (!dataNetworks.length) return;

    setFormValues((prev) => {
      if (prev.dataNetworkName && dataNetworks.includes(prev.dataNetworkName)) {
        return prev;
      }
      return { ...prev, dataNetworkName: dataNetworks[0] };
    });
  }, [dataNetworks]);

  useEffect(() => {
    if (!slices.length) return;

    setFormValues((prev) => {
      if (prev.sliceName && slices.includes(prev.sliceName)) {
        return prev;
      }
      return { ...prev, sliceName: slices[0] };
    });
  }, [slices]);

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });
    try {
      const sessionAmbrUplink = `${formValues.ambrUpValue} ${formValues.ambrUpUnit}`;
      const sessionAmbrDownlink = `${formValues.ambrDownValue} ${formValues.ambrDownUnit}`;
      await createPolicy(accessToken, {
        name: formValues.name,
        profile_name: profileName,
        slice_name: formValues.sliceName,
        data_network_name: formValues.dataNetworkName,
        session_ambr_uplink: sessionAmbrUplink,
        session_ambr_downlink: sessionAmbrDownlink,
        var5qi: formValues.fiveQi,
        arp: formValues.arp,
      });
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
      fullWidth
      maxWidth="sm"
    >
      <DialogTitle id="create-policy-modal-title">Create Policy</DialogTitle>
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
          autoFocus
        />

        <FormControl fullWidth margin="normal">
          <InputLabel id="slice-select-label">Slice</InputLabel>
          <Select
            labelId="slice-select-label"
            label="Slice"
            value={formValues.sliceName}
            onChange={(e) => handleChange("sliceName", e.target.value)}
            onBlur={() => handleBlur("sliceName")}
            error={!!errors.sliceName && touched.sliceName}
          >
            {slices.map((name) => (
              <MenuItem key={name} value={name}>
                {name}
              </MenuItem>
            ))}
          </Select>
          {touched.sliceName && errors.sliceName && (
            <Typography color="error" variant="caption">
              {errors.sliceName}
            </Typography>
          )}
        </FormControl>

        <FormControl fullWidth margin="normal">
          <InputLabel id="data-network-select-label">Data Network</InputLabel>
          <Select
            labelId="data-network-select-label"
            label="Data Network"
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

        <Box sx={{ display: "flex", gap: 2 }}>
          <TextField
            label="Session Bitrate Uplink"
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
            error={!!errors.ambrUpUnit && touched.ambrUpUnit}
            helperText={touched.ambrUpUnit ? errors.ambrUpUnit : ""}
            margin="normal"
          >
            <MenuItem value="Mbps">Mbps</MenuItem>
            <MenuItem value="Gbps">Gbps</MenuItem>
          </TextField>
        </Box>

        <Box sx={{ display: "flex", gap: 2 }}>
          <TextField
            label="Session Bitrate Downlink"
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
            error={!!errors.ambrDownUnit && touched.ambrDownUnit}
            helperText={touched.ambrDownUnit ? errors.ambrDownUnit : ""}
            margin="normal"
          >
            <MenuItem value="Mbps">Mbps</MenuItem>
            <MenuItem value="Gbps">Gbps</MenuItem>
          </TextField>
        </Box>

        <FormControl fullWidth margin="normal">
          <InputLabel id="fiveqi-select-label">5QI</InputLabel>
          <Select
            labelId="fiveqi-select-label"
            label="5QI"
            value={formValues.fiveQi}
            onChange={(e) => handleChange("fiveQi", Number(e.target.value))}
            onBlur={() => handleBlur("fiveQi")}
            error={!!errors.fiveQi && touched.fiveQi}
          >
            {NON_GBR_5QI_OPTIONS.map((opt) => (
              <MenuItem key={opt.value} value={opt.value}>
                {opt.label}
              </MenuItem>
            ))}
          </Select>
          {touched.fiveQi && errors.fiveQi && (
            <Typography color="error" variant="caption">
              {errors.fiveQi}
            </Typography>
          )}
          <Typography variant="caption" color="textSecondary">
            Determines radio scheduling behavior. Only non-GBR classes are
            supported.
          </Typography>
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
              : "Admission control priority at session setup. 1 (highest) to 15 (lowest)."
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
          {loading ? "Creating..." : "Create"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default CreatePolicyModal;
