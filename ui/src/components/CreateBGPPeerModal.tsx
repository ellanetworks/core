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
  Typography,
  Stack,
  IconButton,
  ToggleButton,
  ToggleButtonGroup,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
} from "@mui/material";
import {
  Delete as DeleteIcon,
  Add as AddIcon,
  Lock as LockIcon,
  ExpandMore as ExpandMoreIcon,
  ExpandLess as ExpandLessIcon,
} from "@mui/icons-material";
import * as yup from "yup";
import { ValidationError } from "yup";
import {
  createBGPPeer,
  type BGPImportPrefix,
  type RejectedPrefix,
} from "@/queries/bgp";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

const ipv4Regex =
  /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}$/;

const cidrRegex =
  /^(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}\/\d{1,2}$/;

const schema = yup.object().shape({
  address: yup
    .string()
    .required("Neighbor address is required")
    .matches(ipv4Regex, "Must be a valid IPv4 address"),
  remoteAS: yup
    .number()
    .required("Remote AS is required")
    .min(1, "Must be at least 1")
    .max(4294967295, "Must be at most 4294967295"),
  holdTime: yup
    .number()
    .required("Hold time is required")
    .min(3, "Must be at least 3")
    .max(65535, "Must be at most 65535"),
  password: yup.string(),
  description: yup.string(),
});

type ImportPreset = "none" | "default-route" | "all" | "custom";

function detectPreset(prefixes: BGPImportPrefix[]): ImportPreset {
  if (prefixes.length === 0) return "none";
  if (
    prefixes.length === 1 &&
    prefixes[0].prefix === "0.0.0.0/0" &&
    prefixes[0].maxLength === 0
  )
    return "default-route";
  if (
    prefixes.length === 1 &&
    prefixes[0].prefix === "0.0.0.0/0" &&
    prefixes[0].maxLength === 32
  )
    return "all";
  return "custom";
}

interface CreateBGPPeerModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  rejectedPrefixes?: RejectedPrefix[];
}

type FormValues = {
  address: string;
  remoteAS: number;
  holdTime: number;
  password: string;
  description: string;
};

const CreateBGPPeerModal: React.FC<CreateBGPPeerModalProps> = ({
  open,
  onClose,
  onSuccess,
  rejectedPrefixes = [],
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();
  const [showRejected, setShowRejected] = useState(false);

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState<FormValues>({
    address: "",
    remoteAS: 64512,
    holdTime: 90,
    password: "",
    description: "",
  });

  const [importPrefixes, setImportPrefixes] = useState<BGPImportPrefix[]>([]);
  const [importPreset, setImportPreset] = useState<ImportPreset>("none");

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
  }, [formValues, validateForm]);

  const handlePresetChange = (
    _: React.MouseEvent<HTMLElement>,
    value: ImportPreset | null,
  ) => {
    if (!value) return;
    setImportPreset(value);
    switch (value) {
      case "none":
        setImportPrefixes([]);
        break;
      case "default-route":
        setImportPrefixes([{ prefix: "0.0.0.0/0", maxLength: 0 }]);
        break;
      case "all":
        setImportPrefixes([{ prefix: "0.0.0.0/0", maxLength: 32 }]);
        break;
      case "custom":
        if (importPrefixes.length === 0) {
          setImportPrefixes([{ prefix: "", maxLength: 32 }]);
        }
        break;
    }
  };

  const handleAddPrefix = () => {
    setImportPrefixes((prev) => [...prev, { prefix: "", maxLength: 32 }]);
    setImportPreset("custom");
  };

  const handleRemovePrefix = (index: number) => {
    setImportPrefixes((prev) => {
      const next = prev.filter((_, i) => i !== index);
      setImportPreset(detectPreset(next));
      return next;
    });
  };

  const handlePrefixChange = (
    index: number,
    field: "prefix" | "maxLength",
    value: string | number,
  ) => {
    setImportPrefixes((prev) => {
      const next = [...prev];
      next[index] = { ...next[index], [field]: value };
      setImportPreset(detectPreset(next));
      return next;
    });
  };

  const hasInvalidPrefixes = importPrefixes.some(
    (p) => !cidrRegex.test(p.prefix) || p.maxLength < 0 || p.maxLength > 32,
  );

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });
    try {
      await createBGPPeer(accessToken, {
        address: formValues.address,
        remoteAS: formValues.remoteAS,
        holdTime: formValues.holdTime,
        password: formValues.password || undefined,
        description: formValues.description || undefined,
        importPrefixes: importPrefixes,
      });
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to create BGP peer: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="create-bgp-peer-modal-title"
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle id="create-bgp-peer-modal-title">
        Create BGP Peer
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
          label="Neighbor Address"
          value={formValues.address}
          onChange={(e) => handleChange("address", e.target.value)}
          onBlur={() => handleBlur("address")}
          error={!!errors.address && touched.address}
          helperText={touched.address ? errors.address : ""}
          margin="normal"
          autoFocus
        />
        <TextField
          fullWidth
          label="Remote AS"
          type="number"
          value={formValues.remoteAS}
          onChange={(e) => handleChange("remoteAS", Number(e.target.value))}
          onBlur={() => handleBlur("remoteAS")}
          error={!!errors.remoteAS && touched.remoteAS}
          helperText={touched.remoteAS ? errors.remoteAS : ""}
          margin="normal"
        />
        <TextField
          fullWidth
          label="Hold Time"
          type="number"
          value={formValues.holdTime}
          onChange={(e) => handleChange("holdTime", Number(e.target.value))}
          onBlur={() => handleBlur("holdTime")}
          error={!!errors.holdTime && touched.holdTime}
          helperText={touched.holdTime ? errors.holdTime : ""}
          margin="normal"
        />
        <TextField
          fullWidth
          label="Password"
          type="password"
          value={formValues.password}
          onChange={(e) => handleChange("password", e.target.value)}
          onBlur={() => handleBlur("password")}
          margin="normal"
        />
        <TextField
          fullWidth
          label="Description"
          value={formValues.description}
          onChange={(e) => handleChange("description", e.target.value)}
          onBlur={() => handleBlur("description")}
          margin="normal"
        />

        <Typography variant="subtitle2" sx={{ mt: 3, mb: 1 }}>
          Import Prefix List
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
          Control which routes this peer is allowed to advertise to Ella Core.
        </Typography>

        <ToggleButtonGroup
          value={importPreset}
          exclusive
          onChange={handlePresetChange}
          size="small"
          sx={{ mb: 2 }}
        >
          <ToggleButton value="none">Deny All</ToggleButton>
          <ToggleButton value="default-route">Default Route Only</ToggleButton>
          <ToggleButton value="all">All</ToggleButton>
          <ToggleButton value="custom">Custom</ToggleButton>
        </ToggleButtonGroup>

        {importPreset === "none" && (
          <Typography variant="body2" color="text.secondary">
            All routes from this peer will be rejected.
          </Typography>
        )}
        {importPreset === "default-route" && (
          <Typography variant="body2" color="text.secondary">
            Only the default route (0.0.0.0/0) will be accepted.
          </Typography>
        )}
        {importPreset === "all" && (
          <Typography variant="body2" color="text.secondary">
            All routes will be accepted from this peer.
          </Typography>
        )}

        {importPreset === "custom" && (
          <>
            {importPrefixes.map((entry, index) => (
              <Stack
                key={index}
                direction="row"
                spacing={1}
                sx={{ mb: 1 }}
                alignItems="center"
              >
                <TextField
                  label="Prefix"
                  value={entry.prefix}
                  onChange={(e) =>
                    handlePrefixChange(index, "prefix", e.target.value)
                  }
                  size="small"
                  error={!!entry.prefix && !cidrRegex.test(entry.prefix)}
                  helperText={
                    entry.prefix && !cidrRegex.test(entry.prefix)
                      ? "Must be valid CIDR"
                      : ""
                  }
                  sx={{ flex: 2 }}
                />
                <TextField
                  label="Max Length"
                  type="number"
                  value={entry.maxLength}
                  onChange={(e) =>
                    handlePrefixChange(
                      index,
                      "maxLength",
                      Number(e.target.value),
                    )
                  }
                  size="small"
                  error={entry.maxLength < 0 || entry.maxLength > 32}
                  helperText={
                    entry.maxLength < 0 || entry.maxLength > 32 ? "0–32" : ""
                  }
                  sx={{ flex: 1 }}
                />
                <IconButton
                  size="small"
                  onClick={() => handleRemovePrefix(index)}
                  color="primary"
                >
                  <DeleteIcon fontSize="small" />
                </IconButton>
              </Stack>
            ))}

            <Button
              size="small"
              startIcon={<AddIcon />}
              onClick={handleAddPrefix}
              sx={{ mt: 1 }}
            >
              Add Prefix
            </Button>
          </>
        )}

        {rejectedPrefixes.length > 0 && (
          <>
            <Button
              size="small"
              onClick={() => setShowRejected(!showRejected)}
              startIcon={<LockIcon fontSize="small" />}
              endIcon={
                showRejected ? (
                  <ExpandLessIcon fontSize="small" />
                ) : (
                  <ExpandMoreIcon fontSize="small" />
                )
              }
              sx={{
                justifyContent: "flex-start",
                textTransform: "none",
                mt: 1,
              }}
            >
              {rejectedPrefixes.length} rejected{" "}
              {rejectedPrefixes.length === 1 ? "prefix" : "prefixes"} (system)
            </Button>
            <Collapse in={showRejected}>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                These prefixes are always rejected regardless of import policy.
              </Typography>
              <TableContainer>
                <Table size="small">
                  <TableBody>
                    {rejectedPrefixes.map((f, i) => (
                      <TableRow key={i} sx={{ opacity: 0.7 }}>
                        <TableCell sx={{ fontFamily: "monospace" }}>
                          {f.prefix}
                        </TableCell>
                        <TableCell>{f.description}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            </Collapse>
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={!isValid || loading || hasInvalidPrefixes}
        >
          {loading ? "Creating..." : "Create"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default CreateBGPPeerModal;
