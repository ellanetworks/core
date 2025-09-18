import React, { useCallback, useState, useEffect } from "react";
import {
  Box,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
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
  FormGroup,
} from "@mui/material";
import * as yup from "yup";
import { ValidationError } from "yup";
import { createSubscriber } from "@/queries/subscribers";
import { listPolicies } from "@/queries/policies";
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

type Policy = {
  name: string;
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
      "Sequence Number must be a 6-byte (12-character) hexadecimal string.",
    )
    .required("Sequence Number is required."),
  policyName: yup.string().required("Policy Name is required."),
  opc: yup
    .string()
    .matches(
      /(^$)|(^[0-9a-fA-F]{32}$)/,
      "OPC must be either empty or a 32-character hexadecimal string.",
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

  useEffect(() => {
    const fetchOperatorAndPolicies = async () => {
      if (!accessToken) return;
      try {
        const operator: Operator = await getOperator(accessToken);
        setMcc(operator.id.mcc);
        setMnc(operator.id.mnc);

        const policyData: Policy[] = await listPolicies(accessToken);
        setPolicies(policyData.map((policy) => policy.name));
      } catch (error) {
        console.error("Failed to fetch data:", error);
      }
    };

    if (open) {
      fetchOperatorAndPolicies();
    }
  }, [open, accessToken]);

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
  }, [formValues, validateForm]);

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
      setAlert({
        message: `Failed to create subscriber: ${errorMessage}`,
      });
      console.error("Failed to create subscriber:", error);
    } finally {
      setLoading(false);
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
        <FormGroup
          sx={{ mb: 2, p: 2, border: "1px solid #ccc", borderRadius: 1 }}
        >
          <Typography variant="subtitle1" gutterBottom>
            IMSI
          </Typography>
          <Box display="flex" gap={2} alignItems="center">
            <TextField
              label="MCC"
              value={mcc}
              disabled
              margin="normal"
              sx={{ flex: 1 }}
            />
            <TextField
              label="MNC"
              value={mnc}
              disabled
              margin="normal"
              sx={{ flex: 1 }}
            />
            <TextField
              label="MSIN"
              value={formValues.msin}
              onChange={(e) => handleChange("msin", e.target.value)}
              onBlur={() => handleBlur("msin")}
              error={!!errors.msin && touched.msin}
              helperText={touched.msin ? errors.msin : ""}
              margin="normal"
              sx={{ flex: 2 }}
            />
            <Button
              variant="contained"
              color="primary"
              onClick={generateRandomMSIN}
            >
              Generate
            </Button>
          </Box>
        </FormGroup>
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
          <InputLabel id="demo-simple-select-label">Policy Name</InputLabel>
          <Select
            value={formValues.policyName}
            onChange={(e) => handleChange("policyName", e.target.value)}
            onBlur={() => handleBlur("policyName")}
            error={!!errors.policyName && touched.policyName}
            labelId="demo-simple-select-label"
            label={"PolicyName"}
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
                if (!e.target.checked) {
                  handleChange("opc", "");
                }
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
