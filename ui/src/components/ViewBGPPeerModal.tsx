import React from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Typography,
  Stack,
  Chip,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
} from "@mui/material";
import type { BGPPeer, BGPImportPrefix } from "@/queries/bgp";

function getImportPolicyLabel(prefixes: BGPImportPrefix[] | undefined): string {
  if (!prefixes || prefixes.length === 0) return "None (reject all)";
  if (
    prefixes.length === 1 &&
    prefixes[0].prefix === "0.0.0.0/0" &&
    prefixes[0].maxLength === 0
  )
    return "Default Route Only";
  if (
    prefixes.length === 1 &&
    prefixes[0].prefix === "0.0.0.0/0" &&
    prefixes[0].maxLength === 32
  )
    return "All";
  return "Custom";
}

interface ViewBGPPeerModalProps {
  open: boolean;
  onClose: () => void;
  peer: BGPPeer;
}

const ViewBGPPeerModal: React.FC<ViewBGPPeerModalProps> = ({
  open,
  onClose,
  peer,
}) => {
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
              Address
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
            Import Policy: {policyLabel}
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
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
};

export default ViewBGPPeerModal;
