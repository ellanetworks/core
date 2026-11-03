// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  Stack,
  Typography,
} from "@mui/material";
import {
  defaultNRFAPIRoot,
  diameterPeerLabel,
  formatPLMN,
  n32NodeLabel,
  parseSEPPFQDN,
  type RoamingDiameterPeer,
  type RoamingPartner,
} from "@/queries/roaming";
import N32StatusChip from "@/components/N32StatusChip";

interface ViewRoamingPartnerModalProps {
  open: boolean;
  onClose: () => void;
  partner: RoamingPartner;
  diameterPeers: RoamingDiameterPeer[];
}

const Row = ({ label, value }: { label: string; value: React.ReactNode }) => (
  <Stack direction="row" spacing={2} sx={{ justifyContent: "space-between" }}>
    <Typography variant="body2" color="textSecondary" sx={{ flexShrink: 0 }}>
      {label}
    </Typography>
    <Typography
      variant="body2"
      component="div"
      sx={{ textAlign: "right", wordBreak: "break-all" }}
    >
      {value}
    </Typography>
  </Stack>
);

const ViewRoamingPartnerModal: React.FC<ViewRoamingPartnerModalProps> = ({
  open,
  onClose,
  partner,
  diameterPeers,
}) => {
  const sepp = partner.sepp;
  const nrfPLMN = sepp
    ? (parseSEPPFQDN(sepp.fqdn)?.plmn ?? partner.plmns[0])
    : undefined;
  const peer = partner.diameterPeerID
    ? diameterPeers.find((p) => p.id === partner.diameterPeerID)
    : undefined;

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="view-roaming-partner-title"
      maxWidth="md"
      fullWidth
    >
      <DialogTitle id="view-roaming-partner-title">
        Roaming Partner — {partner.name}
      </DialogTitle>
      <DialogContent dividers>
        <Stack spacing={2}>
          <Row label="Name" value={partner.name} />
          <Row label="PLMNs" value={partner.plmns.map(formatPLMN).join(", ")} />
          <Row label="Profile" value={partner.profileName} />

          <Divider />
          <Typography variant="subtitle2">5G roaming</Typography>
          {sepp ? (
            <>
              <Row label="SEPP FQDN" value={sepp.fqdn} />
              <Row label="N32-c URI" value={sepp.n32cURI} />
              <Row label="N32-f URI" value={sepp.n32fURI || "Same as N32-c"} />
              <Row
                label="NRF API Root"
                value={
                  sepp.nrfAPIRoot ||
                  (nrfPLMN
                    ? `Default (${defaultNRFAPIRoot(nrfPLMN.mcc, nrfPLMN.mnc)})`
                    : "Default")
                }
              />
              <Typography variant="body2" color="textSecondary">
                Trust Anchor
              </Typography>
              <Box
                component="pre"
                aria-label="Trust anchor"
                sx={{
                  m: 0,
                  p: 1,
                  maxHeight: 200,
                  overflow: "auto",
                  fontFamily: "monospace",
                  fontSize: 12,
                  bgcolor: "action.hover",
                  borderRadius: 1,
                }}
              >
                {sepp.trustAnchorPEM.trim()}
              </Box>
              <Typography variant="body2" color="textSecondary">
                N32-c
              </Typography>
              {partner.n32?.length ? (
                <Stack spacing={1} aria-label="N32-c status">
                  {partner.n32.map((entry) => (
                    <Row
                      key={entry.seppID ?? entry.nodeID ?? "local"}
                      label={
                        entry.seppID && entry.nodeID
                          ? `${n32NodeLabel(entry)} (${entry.nodeID})`
                          : n32NodeLabel(entry)
                      }
                      value={
                        <Stack spacing={0.5} sx={{ alignItems: "flex-end" }}>
                          <N32StatusChip n32={entry} withNode={false} />
                          {entry.lastError && (
                            <Typography variant="caption" color="textSecondary">
                              {entry.lastError}
                            </Typography>
                          )}
                        </Stack>
                      }
                    />
                  ))}
                </Stack>
              ) : (
                <Typography variant="body2" color="textSecondary">
                  No node has interfaces.n32.
                </Typography>
              )}
            </>
          ) : (
            <Typography variant="body2" color="textSecondary">
              No 5G roaming agreement.
            </Typography>
          )}

          <Divider />
          <Typography variant="subtitle2">4G roaming</Typography>
          {partner.diameterPeerID ? (
            <Row
              label="Diameter Peer"
              value={peer ? diameterPeerLabel(peer) : partner.diameterPeerID}
            />
          ) : (
            <Typography variant="body2" color="textSecondary">
              No 4G roaming agreement.
            </Typography>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Close</Button>
      </DialogActions>
    </Dialog>
  );
};

export default ViewRoamingPartnerModal;
