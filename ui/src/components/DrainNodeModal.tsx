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
  FormControlLabel,
  Link,
  MenuItem,
  Switch,
  TextField,
  Typography,
} from "@mui/material";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import ExpandLessIcon from "@mui/icons-material/ExpandLess";
import { drainClusterMember, type DrainResponse } from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";

interface Props {
  open: boolean;
  nodeId: number;
  isLeader: boolean;
  onClose: () => void;
  onSuccess: (result: DrainResponse) => void;
}

const DEADLINE_OPTIONS: { label: string; seconds: number }[] = [
  { label: "30 seconds", seconds: 30 },
  { label: "2 minutes", seconds: 120 },
  { label: "10 minutes", seconds: 600 },
  { label: "1 hour", seconds: 3600 },
];

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
  const [showDetails, setShowDetails] = useState(false);
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
          Safely stops new traffic on <strong>node {nodeId}</strong> so it can
          be restarted, upgraded, or removed. Existing flows continue. Use
          Resume to reverse.
        </DialogContentText>

        <Box sx={{ mt: 1 }}>
          <Link
            component="button"
            type="button"
            variant="body2"
            underline="hover"
            onClick={() => setShowDetails((v) => !v)}
            sx={{ display: "inline-flex", alignItems: "center" }}
          >
            {showDetails ? (
              <ExpandLessIcon fontSize="small" />
            ) : (
              <ExpandMoreIcon fontSize="small" />
            )}
            What this does
          </Link>
          <Collapse in={showDetails}>
            <Typography variant="body2" color="textSecondary" sx={{ mt: 1 }}>
              Sets <code>drainState</code> to <em>draining</em>. Sends AMF
              Status Indication so connected RANs treat this node&apos;s GUAMI
              as unavailable. Stops the local BGP speaker.
              {isLeader
                ? " Transfers Raft leadership to another voter before the side-effects run."
                : ""}{" "}
              The node becomes removable once <code>drainState</code> reaches{" "}
              <em>drained</em>.
            </Typography>
          </Collapse>
        </Box>

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
          select
          label="Deadline"
          value={deadlineSeconds}
          onChange={(e) => setDeadlineSeconds(Number(e.target.value))}
          helperText="Wait up to this long for active sessions to clear before marking the node drained."
          margin="normal"
          disabled={immediate}
        >
          {DEADLINE_OPTIONS.map((opt) => (
            <MenuItem key={opt.seconds} value={opt.seconds}>
              {opt.label}
            </MenuItem>
          ))}
        </TextField>
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
