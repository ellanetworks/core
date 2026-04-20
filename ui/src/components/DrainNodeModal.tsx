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
  TextField,
} from "@mui/material";
import { drainNode, type DrainResponse } from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";

interface Props {
  open: boolean;
  nodeId: number;
  isLeader: boolean;
  onClose: () => void;
  onSuccess: (result: DrainResponse) => void;
}

const DrainNodeModal: React.FC<Props> = ({
  open,
  nodeId,
  isLeader,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();
  const [timeoutSeconds, setTimeoutSeconds] = useState<number>(5);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState("");

  const handleConfirm = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert("");
    try {
      const result = await drainNode(accessToken, { timeoutSeconds });
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
          This request targets <strong>node {nodeId}</strong> specifically — it
          is not forwarded to the leader. Drain notifies connected RANs to
          redirect new UE registrations elsewhere and withdraws BGP
          advertisements so upstream routers reroute user-plane traffic.
          {isLeader ? (
            <>
              {" "}
              Because this node is currently the leader, Raft leadership will
              transfer to another voter before the other steps run.
            </>
          ) : null}{" "}
          Existing flows continue until the node is shut down. Other API traffic
          is unaffected.
        </DialogContentText>
        <TextField
          fullWidth
          type="number"
          label="Step timeout (seconds)"
          value={timeoutSeconds}
          onChange={(e) => setTimeoutSeconds(Number(e.target.value))}
          helperText="Maximum time for each step (leadership transfer, RAN notifications, BGP shutdown)."
          margin="normal"
          slotProps={{ htmlInput: { min: 1 } }}
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
