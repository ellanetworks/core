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
  Typography,
} from "@mui/material";
import { registerFleet } from "@/queries/fleet";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface ConfigureFleetModalProps {
  open: boolean;
  onClose: () => void;
}

const ConfigureFleetModal: React.FC<ConfigureFleetModalProps> = ({
  open,
  onClose,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();
  if (!authReady || !accessToken) navigate("/login");

  const [activationToken, setActivationToken] = useState("");
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const handleClose = () => {
    setActivationToken("");
    setAlert({ message: "" });
    onClose();
  };

  const handleSubmit = async () => {
    if (!accessToken) return;
    if (!activationToken.trim()) return;
    setLoading(true);
    setAlert({ message: "" });
    try {
      await registerFleet(accessToken, activationToken.trim());
      handleClose();
    } catch (error: unknown) {
      let errorMessage = "Unknown error occurred.";
      if (error instanceof Error) {
        errorMessage = error.message;
      }
      setAlert({
        message: `Failed to register to fleet: ${errorMessage}`,
      });
      console.error("Failed to register to fleet:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      aria-labelledby="configure-fleet-modal-title"
      aria-describedby="configure-fleet-modal-description"
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle>Configure Fleet</DialogTitle>
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
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          Paste the activation token you received from Ella Fleet to connect
          this instance.
        </Typography>
        <TextField
          fullWidth
          label="Activation Token"
          value={activationToken}
          onChange={(e) => setActivationToken(e.target.value)}
          placeholder="Paste your activation token here"
          multiline
          minRows={3}
          maxRows={6}
          margin="normal"
          disabled={loading}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={loading}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="success"
          onClick={handleSubmit}
          disabled={!activationToken.trim() || loading}
        >
          {loading ? "Connecting..." : "Connect"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ConfigureFleetModal;
