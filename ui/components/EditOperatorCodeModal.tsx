import React, { useState, useEffect } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
  TextField,
  Button,
  Alert,
  Collapse,
} from "@mui/material";
import * as yup from "yup";
import { updateOperatorCode } from "@/queries/operator";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface EditOperatorCodeModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object().shape({
  operatorCode: yup
    .string()
    .required("Operator Code is required.")
    .matches(
      /^[0-9A-Fa-f]{32}$/,
      "Operator Code must be a 32-character hexadecimal string.",
    ),
});

const EditOperatorCodeModal: React.FC<EditOperatorCodeModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const router = useRouter();
  const [cookies, setCookie, removeCookie] = useCookies(["user_token"]);

  if (!cookies.user_token) {
    router.push("/login");
  }

  const [formValues, setFormValues] = useState<{ operatorCode: string }>({
    operatorCode: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({ operatorCode: "" });
      setErrors({});
    }
  }, [open]);

  const handleChange = (field: string, value: string) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));

    // Reset error when the user types
    setErrors((prev) => ({
      ...prev,
      [field]: "",
    }));
  };

  const validate = async (): Promise<boolean> => {
    try {
      await schema.validate(formValues, { abortEarly: false });
      setErrors({});
      return true;
    } catch (err: any) {
      const validationErrors: Record<string, string> = {};
      err.inner.forEach((error: yup.ValidationError) => {
        if (error.path) {
          validationErrors[error.path] = error.message;
        }
      });
      setErrors(validationErrors);
      return false;
    }
  };

  const handleSubmit = async () => {
    const isValid = await validate();
    if (!isValid) {
      return;
    }

    setLoading(true);
    setAlert({ message: "" });

    try {
      await updateOperatorCode(cookies.user_token, formValues.operatorCode);
      onClose();
      onSuccess();
    } catch (error: any) {
      const errorMessage = error?.message || "Unknown error occurred.";
      setAlert({ message: `Failed to update operator code: ${errorMessage}` });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-operator-code-modal-title"
      aria-describedby="edit-operator-code-modal-description"
    >
      <DialogTitle>Edit Operator Code</DialogTitle>
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
        <DialogContentText id="alert-dialog-slide-description">
          The Operator Code (OP) is a secret identifier used to authenticate the
          operator and provision SIM cards. Keep this code secure as it can't be
          retrieved once set.
        </DialogContentText>
        <TextField
          fullWidth
          label="Operator Code"
          value={formValues.operatorCode}
          onChange={(e) => handleChange("operatorCode", e.target.value)}
          error={!!errors.operatorCode}
          helperText={errors.operatorCode}
          margin="normal"
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={loading || !formValues.operatorCode}
        >
          {loading ? "Updating..." : "Update"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default EditOperatorCodeModal;
