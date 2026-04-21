import React, { useEffect, useMemo, useState } from "react";
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
import { useQuery } from "@tanstack/react-query";
import { listClusterMembers, mintClusterJoinToken } from "@/queries/cluster";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";

interface Props {
  open: boolean;
  onClose: () => void;
}

const TTL_OPTIONS: { label: string; seconds: number }[] = [
  { label: "15 minutes", seconds: 15 * 60 },
  { label: "30 minutes", seconds: 30 * 60 },
  { label: "1 hour", seconds: 60 * 60 },
  { label: "4 hours", seconds: 4 * 60 * 60 },
  { label: "24 hours", seconds: 24 * 60 * 60 },
];

const DEFAULT_TTL = 30 * 60;
const MIN_NODE_ID = 1;
const MAX_NODE_ID = 63;

const AddNodeModal: React.FC<Props> = ({ open, onClose }) => {
  const { accessToken } = useAuth();
  const { showSnackbar } = useSnackbar();

  const { data: members } = useQuery({
    queryKey: ["cluster", "members"],
    queryFn: () => listClusterMembers(accessToken!),
    enabled: open && !!accessToken,
  });

  const suggestedNodeId = useMemo(() => {
    const used = new Set((members ?? []).map((m) => m.nodeId));
    for (let id = MIN_NODE_ID; id <= MAX_NODE_ID; id++) {
      if (!used.has(id)) return id;
    }
    return MIN_NODE_ID;
  }, [members]);

  const [nodeId, setNodeId] = useState<number>(suggestedNodeId);
  const [ttlSeconds, setTtlSeconds] = useState<number>(DEFAULT_TTL);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState("");
  const [token, setToken] = useState<string>("");
  const [expiresAt, setExpiresAt] = useState<number>(0);

  useEffect(() => {
    if (open && !token) setNodeId(suggestedNodeId);
  }, [open, suggestedNodeId, token]);

  const nodeIdValid = nodeId >= MIN_NODE_ID && nodeId <= MAX_NODE_ID;
  const nodeIdTaken = (members ?? []).some((m) => m.nodeId === nodeId);

  const handleClose = () => {
    setToken("");
    setExpiresAt(0);
    setAlert("");
    setTtlSeconds(DEFAULT_TTL);
    onClose();
  };

  const handleSubmit = async () => {
    if (!accessToken || !nodeIdValid || nodeIdTaken) return;
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

  const configSnippet = useMemo(
    () => `cluster:\n  node-id: ${nodeId}\n  join-token: ${token}`,
    [nodeId, token],
  );

  const copy = async (text: string, label: string) => {
    if (!navigator.clipboard) {
      showSnackbar(
        "Clipboard API not available. Please use HTTPS or try a different browser.",
        "error",
      );
      return;
    }
    try {
      await navigator.clipboard.writeText(text);
      showSnackbar(`${label} copied to clipboard.`, "success");
    } catch {
      showSnackbar("Failed to copy.", "error");
    }
  };

  const ttlLabel =
    TTL_OPTIONS.find((o) => o.seconds === ttlSeconds)?.label ?? "30 minutes";

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>Add a Node to the Cluster</DialogTitle>
      <DialogContent dividers>
        <Collapse in={!!alert}>
          <Alert severity="error" onClose={() => setAlert("")} sx={{ mb: 2 }}>
            {alert}
          </Alert>
        </Collapse>

        {!token && (
          <>
            <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
              Mint a single-use join token for the new node. The token expires
              in {ttlLabel.toLowerCase()}.
            </Typography>

            <TextField
              fullWidth
              autoFocus
              label="Node ID"
              type="number"
              value={nodeId}
              onChange={(e) => setNodeId(Number(e.target.value))}
              error={!nodeIdValid || nodeIdTaken}
              helperText={
                !nodeIdValid
                  ? `Must be between ${MIN_NODE_ID} and ${MAX_NODE_ID}.`
                  : nodeIdTaken
                    ? "This ID is already in use by another node."
                    : "A unique number identifying the new node in the cluster."
              }
              margin="normal"
            />

            <TextField
              fullWidth
              select
              label="Token lifetime"
              value={ttlSeconds}
              onChange={(e) => setTtlSeconds(Number(e.target.value))}
              helperText="The token can be used once within this window."
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
              Token minted. Copy it now — it is shown only once and expires at{" "}
              {new Date(expiresAt * 1000).toLocaleString()}.
            </Alert>

            <Typography variant="body2" sx={{ mb: 1 }}>
              Add the following to the new node&apos;s configuration file, then
              start it:
            </Typography>

            <Box
              sx={{
                position: "relative",
                p: 1.5,
                pr: 5,
                border: 1,
                borderColor: "divider",
                borderRadius: 1,
                bgcolor: "background.default",
              }}
            >
              <Typography
                component="pre"
                variant="body2"
                sx={{
                  fontFamily: "monospace",
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-all",
                  m: 0,
                }}
              >
                {configSnippet}
              </Typography>
              <Tooltip title="Copy configuration">
                <IconButton
                  size="small"
                  onClick={() => copy(configSnippet, "Configuration")}
                  sx={{ position: "absolute", top: 4, right: 4 }}
                >
                  <ContentCopyIcon fontSize="inherit" />
                </IconButton>
              </Tooltip>
            </Box>

            <Button
              size="small"
              onClick={() => copy(token, "Join token")}
              sx={{ mt: 1 }}
            >
              Copy token only
            </Button>
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
            disabled={!nodeIdValid || nodeIdTaken || loading}
          >
            {loading ? "Minting…" : "Mint Token"}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
};

export default AddNodeModal;
