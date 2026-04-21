import React, { useState } from "react";
import {
  Alert,
  Box,
  Button,
  Collapse,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  MenuItem,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import { mintClusterJoinToken } from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";

interface Props {
  open: boolean;
  onClose: () => void;
}

const TTL_OPTIONS: { label: string; seconds: number }[] = [
  { label: "15 minutes", seconds: 15 * 60 },
  { label: "30 minutes (default)", seconds: 30 * 60 },
  { label: "1 hour", seconds: 60 * 60 },
  { label: "4 hours", seconds: 4 * 60 * 60 },
  { label: "24 hours", seconds: 24 * 60 * 60 },
];

const MintJoinTokenModal: React.FC<Props> = ({ open, onClose }) => {
  const { accessToken } = useAuth();
  const { showSnackbar } = useSnackbar();

  const [nodeId, setNodeId] = useState<number>(2);
  const [ttlSeconds, setTtlSeconds] = useState<number>(30 * 60);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState("");
  const [token, setToken] = useState<string>("");
  const [expiresAt, setExpiresAt] = useState<number>(0);

  const nodeIdValid = nodeId >= 1 && nodeId <= 63;

  const handleClose = () => {
    setToken("");
    setExpiresAt(0);
    setAlert("");
    onClose();
  };

  const handleSubmit = async () => {
    if (!accessToken || !nodeIdValid) return;
    setLoading(true);
    setAlert("");
    try {
      const resp = await mintClusterJoinToken(accessToken, {
        nodeID: nodeId,
        ttlSeconds,
      });
      setToken(resp.token);
      setExpiresAt(resp.expiresAt);
    } catch (err) {
      setAlert(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  };

  const handleCopy = async () => {
    if (!navigator.clipboard) {
      showSnackbar(
        "Clipboard API not available. Please use HTTPS or try a different browser.",
        "error",
      );
      return;
    }
    try {
      await navigator.clipboard.writeText(token);
      showSnackbar("Join token copied to clipboard.", "success");
    } catch {
      showSnackbar("Failed to copy.", "error");
    }
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>Mint Cluster Join Token</DialogTitle>
      <DialogContent dividers>
        <Collapse in={!!alert}>
          <Alert severity="error" onClose={() => setAlert("")} sx={{ mb: 2 }}>
            {alert}
          </Alert>
        </Collapse>

        {!token && (
          <>
            <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
              Mints a single-use HMAC-signed token authorising the given node ID
              to request its first cluster certificate. Paste the token into the
              new node&apos;s <code>cluster.join-token</code> config field
              before starting it — the node will self-register once it obtains
              its leaf.
            </Typography>

            <TextField
              fullWidth
              autoFocus
              label="Node ID"
              type="number"
              value={nodeId}
              onChange={(e) => setNodeId(Number(e.target.value))}
              error={!nodeIdValid}
              helperText={
                !nodeIdValid
                  ? "Node ID must be between 1 and 63"
                  : "Integer from 1 to 63. The issued certificate will carry the SAN spiffe://<cluster-id>/node/<id>."
              }
              margin="normal"
            />

            <TextField
              fullWidth
              select
              label="Token lifetime"
              value={ttlSeconds}
              onChange={(e) => setTtlSeconds(Number(e.target.value))}
              helperText="The token can be used once within this window. After it expires, mint a new one."
              margin="normal"
            >
              {TTL_OPTIONS.map((opt) => (
                <MenuItem key={opt.seconds} value={opt.seconds}>
                  {opt.label}
                </MenuItem>
              ))}
            </TextField>
          </>
        )}

        {token && (
          <>
            <Alert severity="success" sx={{ mb: 2 }}>
              Token minted for node {nodeId}. Copy it now — it is shown only
              once.
            </Alert>

            <Typography variant="body2" color="textSecondary" sx={{ mb: 1 }}>
              Expires at {new Date(expiresAt * 1000).toLocaleString()}.
            </Typography>

            <Box
              sx={{
                display: "flex",
                alignItems: "flex-start",
                gap: 1,
                p: 1.5,
                border: 1,
                borderColor: "divider",
                borderRadius: 1,
                bgcolor: "background.default",
              }}
            >
              <Typography
                variant="body2"
                sx={{
                  fontFamily: "monospace",
                  wordBreak: "break-all",
                  flex: 1,
                }}
              >
                {token}
              </Typography>
              <Tooltip title="Copy token">
                <IconButton size="small" onClick={handleCopy}>
                  <ContentCopyIcon fontSize="inherit" />
                </IconButton>
              </Tooltip>
            </Box>
          </>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleClose} disabled={loading}>
          {token ? "Close" : "Cancel"}
        </Button>
        {!token && (
          <Button
            variant="contained"
            color="success"
            onClick={handleSubmit}
            disabled={!nodeIdValid || loading}
          >
            {loading ? "Minting…" : "Mint Token"}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
};

export default MintJoinTokenModal;
