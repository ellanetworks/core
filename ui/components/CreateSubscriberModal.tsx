import React, { useCallback, useState, useEffect } from "react";
import {
  Box,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  InputAdornment,
  TextField,
  Button,
  Typography,
  Alert,
  Collapse,
  MenuItem,
  FormControlLabel,
  Select,
  Checkbox,
  InputLabel,
  FormControl,
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { createSubscriber } from "@/queries/subscribers";
import {
  listPolicies,
  type APIPolicy,
  type ListPoliciesResponse,
} from "@/queries/policies";
import { getOperator } from "@/queries/operator";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";

interface CreateSubscriberModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

type FormValues = {
  msin: string;
  key: string;
  opc: string;
  sequenceNumber: string;
  policyName: string;
};

type Operator = {
  id: {
    mcc: string;
    mnc: string;
  };
};

const schema = yup.object().shape({
  msin: yup
    .string()
    .length(10, "MSIN must be exactly 10 digits long.")
    .matches(/^\d+$/, "MSIN must be numeric.")
    .required("MSIN is required."),
  key: yup
    .string()
    .matches(
      /^[0-9a-fA-F]{32}$/,
      "Key must be a 32-character hexadecimal string.",
    )
    .required("Key is required."),
  sequenceNumber: yup
    .string()
    .matches(
      /^[0-9a-fA-F]{12}$/,
      "Sequence Number must be a 6-byte (12-char) hex string.",
    )
    .required("Sequence Number is required."),
  policyName: yup.string().required("Policy Name is required."),
  opc: yup
    .string()
    .matches(
      /(^$)|(^[0-9a-fA-F]{32}$)/,
      "OPC must be empty or a 32-character hex string.",
    )
    .notRequired(),
});

const CreateSubscriberModal: React.FC<CreateSubscriberModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const router = useRouter();
  const { accessToken, authReady } = useAuth();

  if (!authReady || !accessToken) {
    router.push("/login");
  }

  const [formValues, setFormValues] = useState<FormValues>({
    msin: "",
    key: "",
    opc: "",
    sequenceNumber: "000000000022",
    policyName: "",
  });

  const [mcc, setMcc] = useState("");
  const [mnc, setMnc] = useState("");
  const [policies, setPolicies] = useState<string[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });
  const [customOPC, setCustomOPC] = useState(false);
  const [imsiMismatch, setImsiMismatch] = useState<string | null>(null);

  useEffect(() => {
    const fetchOperatorAndPolicies = async () => {
      if (!accessToken) return;
      try {
        const operator: Operator = await getOperator(accessToken);
        setMcc(operator.id.mcc);
        setMnc(operator.id.mnc);

        const policyPage: ListPoliciesResponse = await listPolicies(
          accessToken,
          1,
          100,
        );
        setPolicies((policyPage.items ?? []).map((p: APIPolicy) => p.name));
      } catch (error) {
        console.error("Failed to fetch data:", error);
      }
    };

    if (open) {
      fetchOperatorAndPolicies();
    }
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
  }, [formValues, validateForm]);

  useEffect(() => {
    if (!policies.length) return;

    setFormValues((prev) => {
      if (prev.policyName && policies.includes(prev.policyName)) {
        return prev;
      }
      return { ...prev, policyName: policies[0] };
    });
  }, [policies]);

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert({ message: "" });
    try {
      const imsi = `${mcc}${mnc}${formValues.msin}`;
      await createSubscriber(
        accessToken,
        imsi,
        formValues.key,
        formValues.sequenceNumber,
        formValues.policyName,
        formValues.opc,
      );
      onClose();
      onSuccess();
    } catch (error: unknown) {
      const errorMessage =
        error instanceof Error ? error.message : "Unknown error occurred.";
      setAlert({ message: `Failed to create subscriber: ${errorMessage}` });
      console.error("Failed to create subscriber:", error);
    } finally {
      setLoading(false);
    }
  };

  const sanitizeDigits = (s: string) => s.replace(/\D/g, "");

  const parseIMSIorMSIN = (raw: string, mcc: string, mnc: string) => {
    const digits = sanitizeDigits(raw);
    const prefix = `${mcc}${mnc}`;
    const wantLen = prefix.length + 10;

    if (digits.length <= 10) {
      return { msin: digits, mismatchMsg: null };
    }

    if (digits.length >= wantLen) {
      if (digits.startsWith(prefix)) {
        const msin = digits.slice(prefix.length, prefix.length + 10);
        return { msin, mismatchMsg: null };
      }
      return {
        msin: null,
        mismatchMsg: `IMSI prefix does not match MCC ${mcc} / MNC ${mnc}.`,
      };
    }

    return { msin: digits.slice(-10), mismatchMsg: null };
  };

  const handleIMSIishInput = (raw: string) => {
    const { msin, mismatchMsg } = parseIMSIorMSIN(raw, mcc, mnc);

    setImsiMismatch(mismatchMsg);

    if (msin !== null) {
      handleChange("msin", msin);
      setTouched((t) => ({ ...t, msin: true }));
    }
  };

  const generateRandomMSIN = () => {
    const randomMSIN = Math.floor(
      1000000000 + Math.random() * 9000000000,
    ).toString();
    handleChange("msin", randomMSIN);
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="create-subscriber-modal-title"
      aria-describedby="create-subscriber-modal-description"
    >
      <DialogTitle>Create Subscriber</DialogTitle>
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

        <Box display="flex" gap={2} alignItems="center">
          <TextField
            fullWidth
            label="IMSI"
            value={formValues.msin}
            onChange={(e) => handleIMSIishInput(e.target.value)}
            onPaste={(e) => {
              const pasted = e.clipboardData.getData("text");
              if (/\d{12,}/.test(pasted)) {
                e.preventDefault();
                handleIMSIishInput(pasted);
              }
            }}
            onBlur={() => handleBlur("msin")}
            error={(!!errors.msin && touched.msin) || !!imsiMismatch}
            helperText={(touched.msin && errors.msin) || imsiMismatch}
            margin="normal"
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">{`${mcc}${mnc}`}</InputAdornment>
                ),
              },
            }}
          />
          <Button
            variant="contained"
            color="primary"
            sx={{ flex: "0 0 120px", minWidth: 120, flexShrink: 0 }}
            onClick={generateRandomMSIN}
          >
            Generate
          </Button>
        </Box>
        <Box display="flex" gap={2} alignItems="center">
          <TextField
            fullWidth
            label="Key"
            value={formValues.key}
            onChange={(e) => handleChange("key", e.target.value)}
            onBlur={() => handleBlur("key")}
            error={!!errors.key && touched.key}
            helperText={touched.key ? errors.key : ""}
            margin="normal"
            sx={{ flex: 1 }}
          />
          <Button
            variant="contained"
            color="primary"
            sx={{ flex: "0 0 120px", minWidth: 120, flexShrink: 0 }}
            onClick={() => {
              const randomKey = [...Array(32)]
                .map(() => Math.floor(Math.random() * 16).toString(16))
                .join("");
              handleChange("key", randomKey);
            }}
          >
            Generate
          </Button>
        </Box>

        <TextField
          fullWidth
          label="Sequence Number"
          value={formValues.sequenceNumber}
          onChange={(e) => handleChange("sequenceNumber", e.target.value)}
          onBlur={() => handleBlur("sequenceNumber")}
          error={!!errors.sequenceNumber && touched.sequenceNumber}
          helperText={touched.sequenceNumber ? errors.sequenceNumber : ""}
          margin="normal"
        />

        <FormControl fullWidth margin="normal">
          <InputLabel id="policy-select-label">Policy Name</InputLabel>
          <Select
            value={formValues.policyName}
            onChange={(e) => handleChange("policyName", e.target.value)}
            onBlur={() => handleBlur("policyName")}
            error={!!errors.policyName && touched.policyName}
            labelId="policy-select-label"
            label="Policy Name"
          >
            {policies.map((policy) => (
              <MenuItem key={policy} value={policy}>
                {policy}
              </MenuItem>
            ))}
          </Select>
          {touched.policyName && errors.policyName && (
            <Typography color="error" variant="caption">
              {errors.policyName}
            </Typography>
          )}
        </FormControl>

        <FormControlLabel
          control={
            <Checkbox
              checked={customOPC}
              onChange={(e) => {
                setCustomOPC(e.target.checked);
                if (!e.target.checked) handleChange("opc", "");
              }}
            />
          }
          label="Provide custom OPC"
        />

        {customOPC && (
          <TextField
            fullWidth
            label="OPC (optional)"
            value={formValues.opc}
            onChange={(e) => handleChange("opc", e.target.value)}
            onBlur={() => handleBlur("opc")}
            margin="normal"
            helperText="Leave blank to use centrally managed OP"
          />
        )}
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

export default CreateSubscriberModal;
