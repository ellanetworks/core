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
  updateBGPPeer,
  type BGPPeer,
  type BGPImportPrefix,
  type RejectedPrefix,
} from "@/queries/bgp";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import {
  ipRegex,
  cidrRegex,
  detectPreset,
  type ImportPreset,
} from "@/utils/bgp";

const schema = yup.object().shape({
  address: yup
    .string()
    .required("Neighbor address is required")
    .matches(ipRegex, "Must be a valid IP address"),
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

interface EditBGPPeerModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
  peer: BGPPeer;
  rejectedPrefixes?: RejectedPrefix[];
}

type FormValues = {
  address: string;
  remoteAS: number;
  holdTime: number;
  password: string;
  description: string;
};

const EditBGPPeerModal: React.FC<EditBGPPeerModalProps> = ({
  open,
  onClose,
  onSuccess,
  peer,
  rejectedPrefixes = [],
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState<FormValues>({
    address: peer.address,
    remoteAS: peer.remoteAS,
    holdTime: peer.holdTime,
    password: "",
    description: peer.description,
  });

  const [importPrefixes, setImportPrefixes] = useState<BGPImportPrefix[]>(
    peer.importPrefixes ?? [],
  );
  const [importPreset, setImportPreset] = useState<ImportPreset>(
    detectPreset(peer.importPrefixes ?? []),
  );

  const [clearPassword, setClearPassword] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });
  const [showRejected, setShowRejected] = useState(false);

  useEffect(() => {
    if (open) {
      setFormValues({
        address: peer.address,
        remoteAS: peer.remoteAS,
        holdTime: peer.holdTime,
        password: "",
        description: peer.description,
      });
      const prefixes = peer.importPrefixes ?? [];
      setImportPrefixes(prefixes);
      setImportPreset(detectPreset(prefixes));
      setClearPassword(false);
      setErrors({});
      setTouched({});
      setAlert({ message: "" });
    }
  }, [open, peer]);

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
      if (next.length === 0) {
        setImportPreset("none");
      }
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
      let password: string | undefined;
      if (clearPassword) {
        password = "";
      } else if (formValues.password) {
        password = formValues.password;
      }

      await updateBGPPeer(accessToken, peer.id, {
        address: formValues.address,
        remoteAS: formValues.remoteAS,
        holdTime: formValues.holdTime,
        password,
        description: formValues.description || undefined,
        importPrefixes: importPrefixes,
      });
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to update BGP peer: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-bgp-peer-modal-title"
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle id="edit-bgp-peer-modal-title">Edit BGP Peer</DialogTitle>
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
          placeholder="e.g. 10.0.0.1"
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
          helperText={
            touched.remoteAS && errors.remoteAS
              ? errors.remoteAS
              : "Autonomous System number of the remote peer"
          }
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
          helperText={
            touched.holdTime && errors.holdTime
              ? errors.holdTime
              : "Seconds before the session is considered down (3\u201365535)"
          }
          margin="normal"
        />
        <Stack direction="row" spacing={1} alignItems="flex-start">
          <TextField
            fullWidth
            label="Password"
            type="password"
            value={formValues.password}
            onChange={(e) => handleChange("password", e.target.value)}
            onBlur={() => handleBlur("password")}
            margin="normal"
            disabled={clearPassword}
            placeholder={
              peer.hasPassword
                ? "Leave empty to keep current password"
                : "Optional"
            }
            helperText={
              clearPassword
                ? "Password will be removed on save"
                : "TCP MD5 authentication password"
            }
          />
          {peer.hasPassword && (
            <Button
              size="small"
              variant="outlined"
              color={clearPassword ? "primary" : "error"}
              onClick={() => {
                setClearPassword((v) => !v);
                if (!clearPassword) {
                  setFormValues((prev) => ({ ...prev, password: "" }));
                }
              }}
              sx={{ mt: "16px", minWidth: 70, height: 56 }}
            >
              {clearPassword ? "Undo" : "Clear"}
            </Button>
          )}
        </Stack>
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
          <ToggleButton value="all">Accept All</ToggleButton>
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
              sx={{ mt: 1, mb: 2 }}
            >
              Add Prefix
            </Button>
          </>
        )}

        {rejectedPrefixes.length > 0 && (
          <Box sx={{ mt: 1 }}>
            <Button
              size="small"
              startIcon={<LockIcon fontSize="small" />}
              endIcon={
                showRejected ? (
                  <ExpandLessIcon fontSize="small" />
                ) : (
                  <ExpandMoreIcon fontSize="small" />
                )
              }
              onClick={() => setShowRejected((v) => !v)}
              sx={{
                justifyContent: "flex-start",
                textTransform: "none",
                color: "text.secondary",
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
                    {rejectedPrefixes.map((rp) => (
                      <TableRow key={rp.prefix} sx={{ opacity: 0.7 }}>
                        <TableCell sx={{ fontFamily: "monospace" }}>
                          {rp.prefix}
                        </TableCell>
                        <TableCell>{rp.description}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableContainer>
            </Collapse>
          </Box>
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
          {loading ? "Updating..." : "Update"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default EditBGPPeerModal;
