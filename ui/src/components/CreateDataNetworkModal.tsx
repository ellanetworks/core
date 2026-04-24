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
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { createDataNetwork } from "@/queries/data_networks";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { ipv4Regex, ipv6Regex } from "@/utils/bgp";

interface CreateDataNetworkModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const dnnRegex =
  /^(?=.{1,100}$)([a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)(\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$/;

const schema = yup.object().shape({
  name: yup
    .string()
    .matches(
      dnnRegex,
      "Must be a valid DNN (e.g., internet, ims, core.mycompany)",
    )
    .required("Data Network Name is required"),
  ip_pool: yup
    .string()
    .matches(
      /^(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\.(25[0-5]|2[0-4][0-9]|[0-1]?[0-9][0-9]?)\/\d{1,2}$/,
      "Must be a valid IP pool (e.g., 10.45.0.0/22)",
    )
    .required("IP Pool is required"),
  ipv6_pool: yup
    .string()
    .test(
      "ipv6-cidr",
      "Must be a valid IPv6 CIDR with prefix length /48 to /60 (e.g., 2001:db8::/48)",
      (value) => {
        if (!value) return true;
        const match = value.match(
          /^(?:(?:[0-9a-fA-F]{0,4}:){1,7}[0-9a-fA-F]{0,4}|::)(\/\d{1,3})$/,
        );
        if (!match) return false;
        const prefixLen = parseInt(match[1].slice(1), 10);
        return prefixLen >= 48 && prefixLen <= 60;
      },
    ),
  dns: yup
    .string()
    .test("dns-format", "Must be a valid IPv4 or IPv6 address", (value) => {
      if (!value) return false;
      return ipv4Regex.test(value) || ipv6Regex.test(value);
    })
    .required("DNS is required"),
  mtu: yup.number().min(1).max(65535).required("MTU is required"),
});

const CreateDataNetworkModal: React.FC<CreateDataNetworkModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();
  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState({
    name: "",
    ip_pool: "10.45.0.0/22",
    ipv6_pool: "",
    dns: "8.8.8.8",
    mtu: 1456,
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
      await createDataNetwork(
        accessToken,
        formValues.name,
        formValues.ip_pool,
        formValues.dns,
        formValues.mtu,
        formValues.ipv6_pool || undefined,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      let errorMessage = "Unknown error occurred.";
      if (error instanceof Error) {
        errorMessage = error.message;
      }

      setAlert({
        message: `Failed to create data network: ${errorMessage}`,
      });
      console.error("Failed to create data network:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="create-data-network-modal-title"
      aria-describedby="create-data-network-modal-description"
    >
      <DialogTitle id="create-data-network-modal-title">
        Create Data Network
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
        <TextField
          fullWidth
          label="Name (DNN)"
          value={formValues.name}
          onChange={(e) => handleChange("name", e.target.value)}
          onBlur={() => handleBlur("name")}
          error={!!errors.name && touched.name}
          helperText={touched.name ? errors.name : ""}
          margin="normal"
          autoFocus
        />
        <TextField
          fullWidth
          label="IP Pool"
          value={formValues.ip_pool}
          onChange={(e) => handleChange("ip_pool", e.target.value)}
          onBlur={() => handleBlur("ip_pool")}
          error={!!errors.ip_pool && touched.ip_pool}
          helperText={touched.ip_pool ? errors.ip_pool : ""}
          margin="normal"
        />
        <TextField
          fullWidth
          label="IPv6 Pool (optional)"
          value={formValues.ipv6_pool}
          onChange={(e) => handleChange("ipv6_pool", e.target.value)}
          onBlur={() => handleBlur("ipv6_pool")}
          error={!!errors.ipv6_pool && touched.ipv6_pool}
          helperText={touched.ipv6_pool ? errors.ipv6_pool : ""}
          margin="normal"
          placeholder="e.g., 2001:db8::/48"
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

export default CreateDataNetworkModal;
