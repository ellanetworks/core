"use client";

import React, { useState } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Button,
  Alert,
  Collapse,
  FormControlLabel,
  Checkbox,
  Stack,
} from "@mui/material";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";
import { createAPIToken } from "@/queries/api_tokens";

import { LocalizationProvider } from "@mui/x-date-pickers/LocalizationProvider";
import { AdapterDayjs } from "@mui/x-date-pickers/AdapterDayjs";
import { DatePicker } from "@mui/x-date-pickers/DatePicker";
import dayjs, { Dayjs } from "dayjs";

interface CreateAPITokenModalProps {
  open: boolean;
  onClose: () => void;
  onSuccess: (token: string) => void;
}

const CreateAPITokenModal: React.FC<CreateAPITokenModalProps> = ({
  open,
  onClose,
  onSuccess,
}) => {
  const router = useRouter();
  const [cookies] = useCookies(["user_token"]);
  if (!cookies.user_token) {
    router.push("/login");
  }

  const [name, setName] = useState("");
  const [noExpiry, setNoExpiry] = useState(false);
  const [expiry, setExpiry] = useState<Dayjs | null>(null);

  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const reset = () => {
    setName("");
    setNoExpiry(true);
    setExpiry(null);
    setAlert({ message: "" });
  };

  const handleSubmit = async () => {
    if (!name.trim()) {
      setAlert({ message: "Please provide a token name." });
      return;
    }
    try {
      setLoading(true);
      setAlert({ message: "" });

      const expiryISO =
        noExpiry || !expiry ? "" : expiry.endOf("day").toISOString();

      const createResult = await createAPIToken(
        cookies.user_token,
        name.trim(),
        expiryISO,
      );

      reset();
      onClose();
      onSuccess(createResult.token);
    } catch (error: unknown) {
      const msg = error instanceof Error ? error.message : "Unknown error.";
      setAlert({ message: `Failed to create API Token: ${msg}` });
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      aria-labelledby="create-api-token-modal-title"
      aria-describedby="create-api-token-modal-description"
    >
      <DialogTitle id="create-api-token-modal-title">
        Create API Token
      </DialogTitle>
      <DialogContent id="create-api-token-modal-description" dividers>
        <Collapse in={!!alert.message}>
          <Alert
            onClose={() => setAlert({ message: "" })}
            sx={{ mb: 2 }}
            severity="error"
          >
            {alert.message}
          </Alert>
        </Collapse>

        <Stack spacing={2} sx={{ mt: 1, minWidth: { xs: 280, sm: 420 } }}>
          <TextField
            fullWidth
            label="Name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
            margin="normal"
            placeholder="e.g., CI Pipeline, Local Script"
          />

          <LocalizationProvider dateAdapter={AdapterDayjs}>
            <DatePicker
              label="Expiry date"
              value={expiry}
              onChange={(val) => setExpiry(val)}
              disabled={noExpiry}
              minDate={dayjs().startOf("day")}
              slotProps={{
                textField: {
                  fullWidth: true,
                  helperText: noExpiry
                    ? "This token will never expire."
                    : "The token will expire at the end of this day.",
                },
              }}
            />
          </LocalizationProvider>

          <FormControlLabel
            control={
              <Checkbox
                checked={noExpiry}
                onChange={(e) => setNoExpiry(e.target.checked)}
              />
            }
            label="No expiry"
          />
        </Stack>
      </DialogContent>

      <DialogActions>
        <Button onClick={handleClose}>Cancel</Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={loading}
        >
          {loading ? "Creating..." : "Create"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default CreateAPITokenModal;
