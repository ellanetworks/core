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
import { updateDataNetwork, APIDataNetwork } from "@/queries/data_networks";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { ipv4Regex, ipv6Regex, cidrRegex, isValidIpv6Cidr } from "@/utils/bgp";

interface EditDataNetworkModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  initialData: APIDataNetwork;
}

const schema = yup.object().shape({
  ipv4_pool: yup
    .string()
    .test(
      "at-least-one-pool",
      "At least one IP pool (IPv4 or IPv6) is required",
      function (value) {
        const { ipv6_pool } = this.parent;
        if (!value && !ipv6_pool) return false;
        if (!value) return true;
        return cidrRegex.test(value);
      },
    ),
  ipv6_pool: yup
    .string()
    .test(
      "at-least-one-pool",
      "At least one IP pool (IPv4 or IPv6) is required",
      function (value) {
        const { ipv4_pool } = this.parent;
        if (!value && !ipv4_pool) return false;
        if (!value) return true;
        return isValidIpv6Cidr(value);
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

const EditDataNetworkModal: React.FC<EditDataNetworkModalProps> = ({
  open,
  onClose,
  onSuccess,
  initialData,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState(initialData);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({
        name: initialData.name,
        ipv4_pool: initialData.ipv4_pool,
        ipv6_pool: initialData.ipv6_pool || "",
        dns: initialData.dns,
        mtu: initialData.mtu,
      });
      setErrors({});
      setTouched({});
    }
  }, [open, initialData]);

  const handleChange = (field: string, value: string | number) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));
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
      await updateDataNetwork(
        accessToken,
        formValues.name,
        formValues.ipv4_pool,
        formValues.dns,
        formValues.mtu,
        formValues.ipv6_pool || undefined,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({ message: `Failed to update data network: ${errorMessage}` });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-data-network-modal-title"
      aria-describedby="edit-data-network-modal-description"
    >
      <DialogTitle id="edit-data-network-modal-title">
        Edit Data Network
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
          label="Name"
          value={formValues.name}
          margin="normal"
          disabled
        />
        <TextField
          fullWidth
          label="IPv4 Pool"
          value={formValues.ipv4_pool}
          onChange={(e) => handleChange("ipv4_pool", e.target.value)}
          onBlur={() => handleBlur("ipv4_pool")}
          error={!!errors.ipv4_pool && touched.ipv4_pool}
          helperText={touched.ipv4_pool ? errors.ipv4_pool : ""}
          margin="normal"
          autoFocus
        />
        <TextField
          fullWidth
          label="IPv6 Pool"
          value={formValues.ipv6_pool || ""}
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
          {loading ? "Updating..." : "Update"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default EditDataNetworkModal;
