import React, { useState } from "react";
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Collapse,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Link,
  Typography,
} from "@mui/material";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import ExpandLessIcon from "@mui/icons-material/ExpandLess";
import { resumeClusterMember } from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";

interface Props {
  open: boolean;
  nodeId: number;
  onClose: () => void;
  onSuccess: () => void;
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
  const [showCaveats, setShowCaveats] = useState(false);

  const handleConfirm = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert("");
    try {
      await resumeClusterMember(accessToken, nodeId);
      onSuccess();
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
          Clears drain state on <strong>node {nodeId}</strong> and restarts its
          local BGP speaker (if BGP is enabled). Route advertisements resume on
          the next reconciler tick.
        </DialogContentText>

        <Box sx={{ mt: 2 }}>
          <Link
            component="button"
            type="button"
            variant="body2"
            underline="hover"
            onClick={() => setShowCaveats((v) => !v)}
            sx={{ display: "inline-flex", alignItems: "center" }}
          >
            {showCaveats ? (
              <ExpandLessIcon fontSize="small" />
            ) : (
              <ExpandMoreIcon fontSize="small" />
            )}
            What Resume does not reverse
          </Link>
          <Collapse in={showCaveats}>
            <Typography
              component="ul"
              variant="body2"
              color="textSecondary"
              sx={{ mt: 1, pl: 2.5 }}
            >
              <li>
                The AMF Status Indication sent at drain time — RANs treat this
                node&apos;s GUAMI as available again only after the next NG
                Setup (typically on RAN restart or SCTP reconnect).
              </li>
              <li>
                Raft leadership transfer — if this node was the leader when
                drained, it stays a follower until something else moves
                leadership.
              </li>
            </Typography>
          </Collapse>
        </Box>
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
