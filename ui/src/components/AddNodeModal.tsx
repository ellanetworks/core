// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
import { formatDateTime } from "@/utils/formatters";

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

const AddNodeModal: React.FC<Props> = ({ open, onClose }) => {
  const { accessToken } = useAuth();
  const { showSnackbar } = useSnackbar();

  const [ttlSeconds, setTtlSeconds] = useState<number>(DEFAULT_TTL);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState("");
  const [token, setToken] = useState<string>("");
  const [expiresAt, setExpiresAt] = useState<number>(0);

  const handleClose = () => {
    setToken("");
    setExpiresAt(0);
    setAlert("");
    setTtlSeconds(DEFAULT_TTL);
    onClose();
  };

  const handleSubmit = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert("");
    try {
      const resp = await mintClusterJoinToken(accessToken, { ttlSeconds });
      setToken(resp.token);
      setExpiresAt(resp.expiresAt);
    } catch (err) {
      setAlert(err instanceof Error ? err.message : "Unknown error");
    } finally {
      setLoading(false);
    }
  };

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
            <Typography variant="body2" sx={{ mb: 1 }}>
              This will mint a single-use token that admits one new node to the
              cluster. The token must be copied and added to the new node.
            </Typography>

            <TextField
              fullWidth
              autoFocus
              select
              label="Token lifetime"
              value={ttlSeconds}
              onChange={(e) => setTtlSeconds(Number(e.target.value))}
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
            <Typography variant="body2" color="success.main" sx={{ mb: 2 }}>
              Token minted. Copy it now, because it is shown only once. It
              expires at{" "}
              {formatDateTime(new Date(expiresAt * 1000).toISOString())}.
            </Typography>

            <Typography variant="body2" sx={{ mb: 1 }}>
              Open the new node in a browser, click{" "}
              <strong>Join an existing cluster instead</strong> and paste this
              token.
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
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-all",
                  m: 0,
                }}
              >
                {token}
              </Typography>
              <Tooltip title="Copy token">
                <IconButton
                  size="small"
                  onClick={() => copy(token, "Join token")}
                  sx={{ position: "absolute", top: 4, right: 4 }}
                >
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
            disabled={loading}
          >
            {loading ? "Minting…" : "Mint Token"}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
};

export default AddNodeModal;
