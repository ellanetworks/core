import React, { useState } from "react";
import {
  Alert,
  Button,
  CircularProgress,
  Collapse,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControlLabel,
  Switch,
  TextField,
} from "@mui/material";
import { drainClusterMember, type DrainResponse } from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";

interface Props {
  open: boolean;
  nodeId: number;
  isLeader: boolean;
  onClose: () => void;
  onSuccess: (result: DrainResponse) => void;
}

const DEFAULT_DEADLINE = 30;

const DrainNodeModal: React.FC<Props> = ({
  open,
  nodeId,
  isLeader,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();
  const [immediate, setImmediate] = useState<boolean>(false);
  const [deadlineSeconds, setDeadlineSeconds] =
    useState<number>(DEFAULT_DEADLINE);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState("");

  const handleConfirm = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert("");
    try {
      const result = await drainClusterMember(accessToken, nodeId, {
        deadlineSeconds: immediate ? 0 : deadlineSeconds,
      });
      onSuccess(result);
      onClose();
    } catch (err) {
      setAlert(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={loading ? undefined : onClose}
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle>Drain node {nodeId}?</DialogTitle>
      <DialogContent dividers>
        <Collapse in={!!alert}>
          <Alert severity="error" onClose={() => setAlert("")} sx={{ mb: 2 }}>
            {alert}
          </Alert>
        </Collapse>
        <DialogContentText>
          Drain marks <strong>node {nodeId}</strong> as draining and runs the
          drain side-effects on it: connected RANs are told the AMF is
          unavailable, the local BGP speaker stops advertising routes
          {isLeader ? ", and Raft leadership transfers to another voter" : ""}.
          Existing flows continue to be served. The node becomes eligible for
          removal once its drain state reaches <em>drained</em>. Use Resume to
          reverse.
        </DialogContentText>
        <FormControlLabel
          sx={{ mt: 2 }}
          control={
            <Switch
              checked={immediate}
              onChange={(e) => setImmediate(e.target.checked)}
            />
          }
          label="Immediate (don't wait for active sessions to drain)"
        />
        <TextField
          fullWidth
          type="number"
          label="Deadline (seconds)"
          value={deadlineSeconds}
          onChange={(e) => setDeadlineSeconds(Number(e.target.value))}
          helperText="Wait up to this many seconds for the node's active sessions to clear before marking it drained."
          margin="normal"
          disabled={immediate}
          slotProps={{ htmlInput: { min: 0, max: 3600 } }}
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={loading}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="warning"
          onClick={handleConfirm}
          disabled={loading}
          startIcon={
            loading ? <CircularProgress size={16} color="inherit" /> : undefined
          }
        >
          {loading ? "Draining…" : "Drain"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default DrainNodeModal;
