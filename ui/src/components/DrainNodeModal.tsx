// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
import { drainClusterMember, type DrainResponse } from "@/queries/cluster";
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
  const [showDetails, setShowDetails] = useState(false);
  const [loading, setLoading] = useState(false);
  const [alert, setAlert] = useState("");

  const handleConfirm = async () => {
    if (!accessToken) return;
    setLoading(true);
    setAlert("");
    try {
      const result = await drainClusterMember(accessToken, nodeId);
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
          Stops new traffic on <strong>node {nodeId}</strong> and marks it
          drained immediately, so it can be restarted, upgraded, or removed. Use
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
              Sends AMF Status Indication so connected RANs treat this
              node&apos;s GUAMI as unavailable. Stops the local BGP speaker.
              {isLeader
                ? " Transfers Raft leadership to another voter once the side-effects have run."
                : ""}{" "}
              Sets <code>drainState</code> to <em>drained</em>, after which the
              node is removable.
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
