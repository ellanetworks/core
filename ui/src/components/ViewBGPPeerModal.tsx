import React, { useState } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Typography,
  Stack,
  Chip,
  Collapse,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
} from "@mui/material";
import {
  Lock as LockIcon,
  ExpandMore as ExpandMoreIcon,
  ExpandLess as ExpandLessIcon,
} from "@mui/icons-material";
import type { BGPPeer, BGPImportPrefix, RejectedPrefix } from "@/queries/bgp";
import { getImportPolicyLabel } from "@/utils/bgp";

interface ViewBGPPeerModalProps {
  open: boolean;
  onClose: () => void;
  peer: BGPPeer;
  rejectedPrefixes?: RejectedPrefix[];
}

const ViewBGPPeerModal: React.FC<ViewBGPPeerModalProps> = ({
  open,
  onClose,
  peer,
  rejectedPrefixes = [],
}) => {
  const [showRejected, setShowRejected] = useState(false);
  const policyLabel = getImportPolicyLabel(peer.importPrefixes);
  const state = peer.state;
  const statusLabel = state
    ? state.charAt(0).toUpperCase() + state.slice(1)
    : "Unknown";
  const statusText =
    state === "established" && peer.uptime
      ? `${statusLabel} (${peer.uptime})`
      : statusLabel;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="view-bgp-peer-modal-title"
      maxWidth="sm"
      fullWidth
    >
      <DialogTitle id="view-bgp-peer-modal-title">
        BGP Peer — {peer.address}
      </DialogTitle>
      <DialogContent dividers>
        <Stack spacing={2}>
          <Stack direction="row" justifyContent="space-between">
            <Typography variant="body2" color="text.secondary">
              Neighbor Address
            </Typography>
            <Typography variant="body2" fontFamily="monospace">
              {peer.address}
            </Typography>
          </Stack>

          <Stack direction="row" justifyContent="space-between">
            <Typography variant="body2" color="text.secondary">
              Remote AS
            </Typography>
            <Typography variant="body2">{peer.remoteAS}</Typography>
          </Stack>

          <Stack direction="row" justifyContent="space-between">
            <Typography variant="body2" color="text.secondary">
              Hold Time
            </Typography>
            <Typography variant="body2">{peer.holdTime}s</Typography>
          </Stack>

          {peer.description && (
            <Stack direction="row" justifyContent="space-between">
              <Typography variant="body2" color="text.secondary">
                Description
              </Typography>
              <Typography variant="body2">{peer.description}</Typography>
            </Stack>
          )}

          <Stack direction="row" justifyContent="space-between">
            <Typography variant="body2" color="text.secondary">
              Password
            </Typography>
            <Typography variant="body2">
              {peer.hasPassword ? "Configured" : "Not set"}
            </Typography>
          </Stack>

          <Stack
            direction="row"
            justifyContent="space-between"
            alignItems="center"
          >
            <Typography variant="body2" color="text.secondary">
              Status
            </Typography>
            <Chip
              label={statusText}
              color={state === "established" ? "success" : "default"}
              size="small"
            />
          </Stack>

          <Stack direction="row" justifyContent="space-between">
            <Typography variant="body2" color="text.secondary">
              Prefixes Sent
            </Typography>
            <Typography variant="body2">{peer.prefixesSent ?? 0}</Typography>
          </Stack>

          <Stack direction="row" justifyContent="space-between">
            <Typography variant="body2" color="text.secondary">
              Prefixes Received
            </Typography>
            <Typography variant="body2">
              {peer.prefixesReceived ?? 0}
            </Typography>
          </Stack>

          <Stack direction="row" justifyContent="space-between">
            <Typography variant="body2" color="text.secondary">
              Prefixes Accepted
            </Typography>
            <Typography variant="body2">
              {peer.prefixesAccepted ?? 0}
            </Typography>
          </Stack>

          <Typography variant="subtitle2" sx={{ mt: 1 }}>
            Import Prefix List: {policyLabel}
          </Typography>

          {peer.importPrefixes && peer.importPrefixes.length > 0 && (
            <TableContainer>
              <Table size="small">
                <TableBody>
                  {peer.importPrefixes.map((p: BGPImportPrefix, i: number) => (
                    <TableRow key={i}>
                      <TableCell sx={{ fontFamily: "monospace" }}>
                        {p.prefix}
                      </TableCell>
                      <TableCell>max /{p.maxLength}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}

          {rejectedPrefixes.length > 0 && (
            <>
              <Button
                size="small"
                onClick={() => setShowRejected(!showRejected)}
                startIcon={<LockIcon fontSize="small" />}
                endIcon={
                  showRejected ? (
                    <ExpandLessIcon fontSize="small" />
                  ) : (
                    <ExpandMoreIcon fontSize="small" />
                  )
                }
                sx={{
                  justifyContent: "flex-start",
                  textTransform: "none",
                  color: "text.secondary",
                }}
              >
                {rejectedPrefixes.length} rejected{" "}
                {rejectedPrefixes.length === 1 ? "prefix" : "prefixes"} (system)
              </Button>
              <Collapse in={showRejected}>
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mb: 1 }}
                >
                  These prefixes are always rejected regardless of import
                  policy.
                </Typography>
                <TableContainer>
                  <Table size="small">
                    <TableBody>
                      {rejectedPrefixes.map((f, i) => (
                        <TableRow key={i} sx={{ opacity: 0.7 }}>
                          <TableCell sx={{ fontFamily: "monospace" }}>
                            {f.prefix}
                          </TableCell>
                          <TableCell>{f.description}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
              </Collapse>
            </>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
};

export default ViewBGPPeerModal;
