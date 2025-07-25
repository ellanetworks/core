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
import { updateOperatorHomeNetwork } from "@/queries/operator";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";

interface EditOperatorHomeNetworkModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const schema = yup.object().shape({
  privateKey: yup
    .string()
    .matches(
      /^[a-fA-F0-9]{64}$/,
      "Private Key must be a 64-character hexadecimal string.",
    )
    .required("Private Key is required."),
});

const EditOperatorHomeNetworkModal: React.FC<
  EditOperatorHomeNetworkModalProps
> = ({ open, onClose, onSuccess }) => {
  const router = useRouter();
  const [cookies, setCookie, removeCookie] = useCookies(["user_token"]);

  if (!cookies.user_token) {
    router.push("/login");
  }

  const [formValues, setFormValues] = useState<{ privateKey: string }>({
    privateKey: "",
  });
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  useEffect(() => {
    if (open) {
      setFormValues({ privateKey: "" });
      setErrors({});
    }
  }, [open]);

  const handleChange = (field: string, value: string) => {
    setFormValues((prev) => ({
      ...prev,
      [field]: value,
    }));

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
      await updateOperatorHomeNetwork(
        cookies.user_token,
        formValues.privateKey,
      );
      onClose();
      onSuccess();
    } catch (error: any) {
      const errorMessage = error?.message || "Unknown error occurred.";
      setAlert({
        message: `Failed to update operator home network information: ${errorMessage}`,
      });
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="edit-operator-home-network-modal-title"
      aria-describedby="edit-operator-home-network-modal-description"
    >
      <DialogTitle>Edit Operator Home Network Information</DialogTitle>
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
          The Home Network Private Key ensures IMSI privacy. User Equipment (UE)
          devices will use the public key to encrypt the IMSI before sending it
          to the network. The network will then use the private key to decrypt
          the IMSI.
        </DialogContentText>
        <TextField
          fullWidth
          label="Private Key"
          value={formValues.privateKey}
          onChange={(e) => handleChange("privateKey", e.target.value)}
          error={!!errors.privateKey}
          helperText={errors.privateKey}
          margin="normal"
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={loading || !formValues.privateKey}
        >
          {loading ? "Updating..." : "Update"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default EditOperatorHomeNetworkModal;
