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
} from "@mui/material";
import { resumeClusterMember, type ResumeResponse } from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";

interface Props {
  open: boolean;
  nodeId: number;
  onClose: () => void;
  onSuccess: (result: ResumeResponse) => void;
}

const ResumeNodeModal: React.FC<Props> = ({
  open,
  nodeId,
  onClose,
  onSuccess,
}) => {
  const { accessToken } = useAuth();
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState("");

  const handleConfirm = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert("");
    try {
      const result = await resumeClusterMember(accessToken, nodeId);
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
      <DialogTitle>Resume node {nodeId}?</DialogTitle>
      <DialogContent dividers>
        <Collapse in={!!alert}>
          <Alert severity="error" onClose={() => setAlert("")} sx={{ mb: 2 }}>
            {alert}
          </Alert>
        </Collapse>
        <DialogContentText>
          Resume clears the drain state on <strong>node {nodeId}</strong> and
          restarts its local BGP speaker (if BGP is enabled). The node will
          start advertising routes again on the reconciler's next tick.
        </DialogContentText>
        <DialogContentText sx={{ mt: 2 }}>
          Resume does <strong>not</strong> reverse:
          <ul>
            <li>
              The AMF Status Indication sent at drain time — RANs will only
              treat this node's GUAMI as available again after the next NG Setup
              (typically on RAN restart or SCTP reconnect).
            </li>
            <li>
              Raft leadership transfer — if this node was the leader when
              drained, it remains a follower until something else moves
              leadership.
            </li>
          </ul>
        </DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={loading}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="primary"
          onClick={handleConfirm}
          disabled={loading}
          startIcon={
            loading ? <CircularProgress size={16} color="inherit" /> : undefined
          }
        >
          {loading ? "Resuming…" : "Resume"}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ResumeNodeModal;
