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
  listProfiles,
  type APIProfile,
  type ListProfilesResponse,
} from "@/queries/profiles";
import { getOperator } from "@/queries/operator";
import { useNavigate } from "react-router-dom";
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
  profileName: string;
};

type Operator = {
  id: {
    mcc: string;
    mnc: string;
  };
};

const getMSINLength = (mnc: string) => (mnc?.length === 3 ? 9 : 10);

const schema = yup.object().shape({
  msin: yup
    .string()
    .matches(/^\d+$/, "MSIN must be numeric.")
    .test("msin-len", function (value) {
      const mncLength = this.options?.context?.mncLength ?? 2;
      const len = mncLength === 3 ? 9 : 10;
      if (!value) return this.createError({ message: "MSIN is required." });
      return (
        value.length === len ||
        this.createError({
          message: `MSIN must be exactly ${len} digits long.`,
        })
      );
    })
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
  profileName: yup.string().required("Profile is required."),
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
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (!authReady || !accessToken) {
      navigate("/login");
    }
  }, [authReady, accessToken, navigate]);

  const [formValues, setFormValues] = useState<FormValues>({
    msin: "",
    key: "",
    opc: "",
    sequenceNumber: "000000000022",
    profileName: "",
  });

  const [mcc, setMcc] = useState("");
  const [mnc, setMnc] = useState("");
  const [profiles, setProfiles] = useState<string[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [isValid, setIsValid] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });
  const [customOPC, setCustomOPC] = useState(false);
  const [imsiMismatch, setImsiMismatch] = useState<string | null>(null);

  useEffect(() => {
    const fetchOperatorAndProfiles = async () => {
      if (!accessToken) return;
      try {
        const operator: Operator = await getOperator(accessToken);
        setMcc(operator.id.mcc);
        setMnc(operator.id.mnc);

        const profilePage: ListProfilesResponse = await listProfiles(
          accessToken,
          1,
          100,
        );
        setProfiles((profilePage.items ?? []).map((p: APIProfile) => p.name));
      } catch (error) {
        console.error("Failed to fetch data:", error);
        setAlert({
          message: "Failed to load operator or profile data. Please try again.",
        });
      }
    };

    if (open) {
      fetchOperatorAndProfiles();
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
      await schema.validateAt(
        field,
        { ...formValues, [field]: value },
        { context: { mncLength: mnc.length } },
      );
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
      await schema.validate(formValues, {
        abortEarly: false,
        context: { mncLength: mnc.length },
      });
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
  }, [formValues, mnc.length]);

  useEffect(() => {
    validateForm();
  }, [formValues, validateForm]);

  useEffect(() => {
    if (!profiles.length) return;

    setFormValues((prev) => {
      if (prev.profileName && profiles.includes(prev.profileName)) {
        return prev;
      }
      return { ...prev, profileName: profiles[0] };
    });
  }, [profiles]);

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
        formValues.profileName,
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
    const msinLen = 15 - (mcc.length + mnc.length); // -> 10 if MNC=2, 9 if MNC=3
    const fullLen = prefix.length + msinLen;

    // 1) Plain MSIN entry
    if (digits.length <= msinLen) {
      return { msin: digits, mismatchMsg: null };
    }

    // 2) Full IMSI (or longer)
    if (digits.length >= fullLen) {
      if (digits.startsWith(prefix)) {
        const msin = digits.slice(prefix.length, prefix.length + msinLen);
        return { msin, mismatchMsg: null };
      }
      return {
        msin: null,
        mismatchMsg: `IMSI prefix does not match MCC ${mcc} / MNC ${mnc}.`,
      };
    }

    // 3) Partial IMSI in progress
    return { msin: digits.slice(-msinLen), mismatchMsg: null };
  };

  const handleIMSIishInput = (raw: string) => {
    const { msin, mismatchMsg } = parseIMSIorMSIN(raw, mcc, mnc);

    setImsiMismatch(mismatchMsg);

    if (msin !== null) {
      handleChange("msin", msin);
      setTouched((t) => ({ ...t, msin: true }));
    }
  };

  const randomDigits = (len: number) =>
    Array.from({ length: len }, () => Math.floor(Math.random() * 10)).join("");

  const generateRandomMSIN = () => {
    const len = getMSINLength(mnc); // 9 if MNC is 3 digits, else 10
    const randomMSIN = randomDigits(len);
    handleChange("msin", randomMSIN);
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="create-subscriber-modal-title"
      aria-describedby="create-subscriber-modal-description"
      fullWidth
      maxWidth="sm"
    >
      <DialogTitle id="create-subscriber-modal-title">
        Create Subscriber
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

        <Box display="flex" gap={2} alignItems="flex-start">
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
            helperText={(touched.msin && errors.msin) || imsiMismatch || " "}
            margin="normal"
            autoFocus
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
            sx={{
              flex: "0 0 120px",
              minWidth: 120,
              flexShrink: 0,
              mt: "16px",
              height: "56px",
            }}
            onMouseDown={(e) => e.preventDefault()}
            onClick={generateRandomMSIN}
          >
            Generate
          </Button>
        </Box>
        <Box display="flex" gap={2} alignItems="flex-start">
          <TextField
            fullWidth
            label="Key"
            value={formValues.key}
            onChange={(e) => handleChange("key", e.target.value)}
            onBlur={() => handleBlur("key")}
            error={!!errors.key && touched.key}
            helperText={touched.key ? errors.key || " " : " "}
            margin="normal"
            sx={{ flex: 1 }}
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
          <InputLabel id="profile-select-label">Profile</InputLabel>
          <Select
            value={formValues.profileName}
            onChange={(e) => handleChange("profileName", e.target.value)}
            onBlur={() => handleBlur("profileName")}
            error={!!errors.profileName && touched.profileName}
            labelId="profile-select-label"
            label="Profile"
          >
            {profiles.map((profile) => (
              <MenuItem key={profile} value={profile}>
                {profile}
              </MenuItem>
            ))}
          </Select>
          {touched.profileName && errors.profileName && (
            <Typography color="error" variant="caption">
              {errors.profileName}
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
            error={!!errors.opc && touched.opc}
            helperText={
              touched.opc && errors.opc
                ? errors.opc
                : "Leave blank to use centrally managed OP"
            }
            margin="normal"
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
