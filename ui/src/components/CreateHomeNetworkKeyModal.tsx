import React, { useCallback, useState, useEffect } from "react";
import {
  Box,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  TextField,
  Button,
  Alert,
  Collapse,
  MenuItem,
  Select,
  InputLabel,
  FormControl,
  IconButton,
  InputAdornment,
} from "@mui/material";
import { ContentCopy as CopyIcon } from "@mui/icons-material";
import * as yup from "yup";
import { createHomeNetworkKey } from "@/queries/operator";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";

interface CreateHomeNetworkKeyModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

type FormValues = {
  keyIdentifier: string;
  scheme: "A" | "B";
  privateKey: string;
};

const schema = yup.object().shape({
  keyIdentifier: yup
    .number()
    .typeError("Key Identifier must be a number.")
    .min(0, "Key Identifier must be between 0 and 255.")
    .max(255, "Key Identifier must be between 0 and 255.")
    .required("Key Identifier is required."),
  scheme: yup
    .string()
    .oneOf(["A", "B"], 'Scheme must be "A" or "B".')
    .required("Scheme is required."),
  privateKey: yup
    .string()
    .matches(
      /^[a-fA-F0-9]{64}$/,
      "Private Key must be a 64-character hexadecimal string.",
    )
    .required("Private Key is required."),
});

const CreateHomeNetworkKeyModal: React.FC<CreateHomeNetworkKeyModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState<FormValues>({
    keyIdentifier: "0",
    scheme: "A",
    privateKey: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({ keyIdentifier: "0", scheme: "A", privateKey: "" });
      setErrors({});
      setTouched({});
      setAlert({ message: "" });
    }
  }, [open]);

  const handleChange = (field: string, value: string) => {
    setFormValues((prev) => ({ ...prev, [field]: value }));
    validateField(field, value);
  };

  const handleBlur = (field: string) => {
    setTouched((prev) => ({ ...prev, [field]: true }));
  };

  const validateField = async (field: string, value: string) => {
    try {
      await schema.validateAt(field, { ...formValues, [field]: value });
      setErrors((prev) => {
        const next = { ...prev };
        delete next[field];
        return next;
      });
    } catch (err) {
      if (err instanceof yup.ValidationError) {
        setErrors((prev) => ({ ...prev, [field]: err.message }));
      }
    }
  };

  const validateForm = useCallback(async () => {
    try {
      await schema.validate(
        {
          ...formValues,
          keyIdentifier: Number(formValues.keyIdentifier),
        },
        { abortEarly: false },
      );
      setIsValid(true);
    } catch {
      setIsValid(false);
    }
  }, [formValues]);

  useEffect(() => {
    validateForm();
  }, [validateForm]);

  const generatePrivateKey = () => {
    const bytes = new Uint8Array(32);
    crypto.getRandomValues(bytes);
    const hex = Array.from(bytes)
      .map((b) => b.toString(16).padStart(2, "0"))
      .join("");
    handleChange("privateKey", hex);
    setTouched((prev) => ({ ...prev, privateKey: true }));
  };

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });
    try {
      await createHomeNetworkKey(
        accessToken,
        Number(formValues.keyIdentifier),
        formValues.scheme,
        formValues.privateKey,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({
        message: `Failed to create home network key: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="create-home-network-key-modal-title"
      fullWidth
      maxWidth="sm"
    >
      <DialogTitle id="create-home-network-key-modal-title">
        Add Home Network Key
      </DialogTitle>
      <DialogContent dividers>
        <DialogContentText sx={{ mb: 1 }}>
          Configure a home network key for SUCI de-concealment. The key
          identifier and scheme must match the values provisioned on the
          subscriber&apos;s SIM card.
        </DialogContentText>
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
          label="Key Identifier"
          type="number"
          value={formValues.keyIdentifier}
          onChange={(e) => handleChange("keyIdentifier", e.target.value)}
          onBlur={() => handleBlur("keyIdentifier")}
          error={touched.keyIdentifier && !!errors.keyIdentifier}
          helperText={
            touched.keyIdentifier && errors.keyIdentifier
              ? errors.keyIdentifier
              : "0-255. Must match the value provisioned on the SIM/USIM."
          }
          margin="normal"
          autoFocus
          slotProps={{ input: { inputProps: { min: 0, max: 255 } } }}
        />

        <FormControl fullWidth margin="normal">
          <InputLabel id="scheme-select-label">Scheme</InputLabel>
          <Select
            labelId="scheme-select-label"
            label="Scheme"
            value={formValues.scheme}
            onChange={(e) => handleChange("scheme", e.target.value)}
            onBlur={() => handleBlur("scheme")}
            error={touched.scheme && !!errors.scheme}
          >
            <MenuItem value="A">Profile A (X25519)</MenuItem>
            <MenuItem value="B">Profile B (P-256)</MenuItem>
          </Select>
        </FormControl>

        <Box display="flex" gap={2} alignItems="flex-start">
          <TextField
            fullWidth
            label="Private Key"
            value={formValues.privateKey}
            onChange={(e) => handleChange("privateKey", e.target.value)}
            onBlur={() => handleBlur("privateKey")}
            error={touched.privateKey && !!errors.privateKey}
            helperText={
              touched.privateKey && errors.privateKey ? errors.privateKey : " "
            }
            margin="normal"
            sx={{
              flex: 1,
              "& .MuiInputBase-input": {
                textOverflow: "ellipsis",
                overflow: "hidden",
                whiteSpace: "nowrap",
              },
            }}
            slotProps={{
              input: {
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      edge="end"
                      onClick={() => {
                        if (!navigator.clipboard) {
                          showSnackbar("Clipboard API not available.", "error");
                          return;
                        }
                        navigator.clipboard
                          .writeText(formValues.privateKey)
                          .then(
                            () =>
                              showSnackbar(
                                "Private key copied to clipboard.",
                                "success",
                              ),
                            () =>
                              showSnackbar(
                                "Failed to copy private key.",
                                "error",
                              ),
                          );
                      }}
                      disabled={!formValues.privateKey}
                    >
                      <CopyIcon
                        fontSize="small"
                        color={formValues.privateKey ? "primary" : "disabled"}
                      />
                    </IconButton>
                  </InputAdornment>
                ),
              },
            }}
          />
          <Button
            variant="contained"
            color="primary"
            sx={{
              flex: "0 0 120px",
              minWidth: 120,
              flexShrink: 0,
              mt: "16px",
              height: "56px",
            }}
            onMouseDown={(e) => e.preventDefault()}
            onClick={generatePrivateKey}
          >
            Generate
          </Button>
        </Box>

        <Alert severity="warning" sx={{ mt: 1 }}>
          Save this private key now. It will not be retrievable after creation.
        </Alert>
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

export default CreateHomeNetworkKeyModal;
