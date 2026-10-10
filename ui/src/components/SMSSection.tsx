// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Box,
  Button,
  Chip,
  IconButton,
  Skeleton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import {
  Add as AddIcon,
  Delete as DeleteIcon,
  Edit as EditIcon,
} from "@mui/icons-material";
import {
  deleteSMSCPeer,
  listSMSCPeers,
  type OperatorSMS,
  type SMSCPeer,
  type SMSCPeerState,
} from "@/queries/operator";
import EditOperatorSMSModal from "@/components/EditOperatorSMSModal";
import SMSCPeerModal from "@/components/SMSCPeerModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { TABLE_CONTAINER_SX } from "@/utils/layout";
import { formatDateTime } from "@/utils/formatters";

const stateLabels: Record<
  SMSCPeerState,
  { label: string; color: "success" | "warning" | "error" | "default" }
> = {
  open: { label: "Connected", color: "success" },
  connecting: { label: "Connecting", color: "warning" },
  reopen: { label: "Reconnecting", color: "warning" },
  suspect: { label: "Unresponsive", color: "warning" },
  closing: { label: "Disconnecting", color: "warning" },
  down: { label: "Disconnected", color: "error" },
};

const formatEndpoint = (address: string, port: number) =>
  address.includes(":") ? `[${address}]:${port}` : `${address}:${port}`;

const SMSCStatus: React.FC<{ peer: SMSCPeer }> = ({ peer }) => {
  const status = peer.status;
  const { label, color } = status
    ? (stateLabels[status.state] ?? { label: status.state, color: "default" })
    : stateLabels.connecting;

  const tooltip =
    status?.state === "open"
      ? [
          status.host
            ? `Connected to ${[status.host, status.realm].filter(Boolean).join(" in realm ")}`
            : "",
          `since ${formatDateTime(status.since)}.`,
        ]
          .filter(Boolean)
          .join(" ")
      : (status?.error ?? "");

  return (
    <Tooltip title={tooltip} arrow>
      <Chip label={label} color={color} size="small" variant="outlined" />
    </Tooltip>
  );
};

interface SMSSectionProps {
  sms?: OperatorSMS;
  canEdit: boolean;
  onModalOpenChange: (open: boolean) => void;
}

const SMSSection: React.FC<SMSSectionProps> = ({
  sms,
  canEdit,
  onModalOpenChange,
}) => {
  const theme = useTheme();
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const queryClient = useQueryClient();

  const [numberModalOpen, setNumberModalOpen] = useState(false);
  const [peerModal, setPeerModal] = useState<{ peer?: SMSCPeer } | null>(null);
  const [peerToDelete, setPeerToDelete] = useState<SMSCPeer | null>(null);

  const anyModalOpen =
    numberModalOpen || peerModal !== null || peerToDelete !== null;

  useEffect(() => {
    onModalOpenChange(anyModalOpen);
  }, [anyModalOpen, onModalOpenChange]);

  const peersQuery = useQuery<SMSCPeer[]>({
    queryKey: ["smsc-peers"],
    enabled: authReady && !!accessToken && !anyModalOpen,
    refetchInterval: 5000,
    queryFn: () => listSMSCPeers(accessToken!),
    placeholderData: (prev) => prev,
  });

  const peers = peersQuery.data ?? [];
  const peersLoading = peersQuery.isLoading && !peersQuery.data;
  const hasNumber = !!sms?.smsNumber;
  const ready = peers.length > 0 && hasNumber;

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ["operator"] });
    queryClient.invalidateQueries({ queryKey: ["smsc-peers"] });
  };

  const handleDelete = async () => {
    if (!peerToDelete || !accessToken) return;
    try {
      await deleteSMSCPeer(accessToken, peerToDelete.id);
      refresh();
      showSnackbar("Service center deleted successfully.", "success");
    } catch (error) {
      const msg =
        error instanceof Error ? error.message : "Unknown error occurred.";
      showSnackbar(`Failed to delete service center: ${msg}`, "error");
    } finally {
      setPeerToDelete(null);
    }
  };

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h6" sx={{ mb: 2 }}>
        SMS
      </Typography>
      <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
        Let 4G and 5G subscribers send and receive SMS. Each subscriber needs a
        phone number.
      </Typography>

      <TableContainer sx={{ ...TABLE_CONTAINER_SX, mb: 3 }}>
        <Table>
          <TableBody>
            <TableRow>
              <TableCell sx={{ fontWeight: 600, width: "35%" }}>
                <Tooltip title="This network's E.164 number for SMS" arrow>
                  <span>SMS Number</span>
                </Tooltip>
              </TableCell>
              <TableCell sx={{ width: "55%" }}>
                {sms?.smsNumber || "N/A"}
              </TableCell>
              <TableCell sx={{ width: "10%", textAlign: "right" }}>
                {canEdit && (
                  <Tooltip title="Edit SMS number" arrow>
                    <IconButton
                      size="small"
                      onClick={() => setNumberModalOpen(true)}
                      aria-label="Edit SMS number"
                    >
                      <EditIcon fontSize="small" color="primary" />
                    </IconButton>
                  </Tooltip>
                )}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </TableContainer>

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
        Service Centers
      </Typography>
      <Stack
        direction="row"
        sx={{ alignItems: "center", justifyContent: "space-between", mb: 2 }}
      >
        <Typography variant="body2" color="textSecondary">
          SMSCs that store and deliver your subscribers&apos; messages.
        </Typography>
        {canEdit && (
          <Button
            variant="contained"
            color="success"
            size="small"
            startIcon={<AddIcon />}
            onClick={() => setPeerModal({})}
            aria-label="Add service center"
            sx={{ ml: 2, flexShrink: 0 }}
          >
            Add Service Center
          </Button>
        )}
      </Stack>
      <TableContainer sx={TABLE_CONTAINER_SX}>
        <Table sx={{ tableLayout: "fixed" }}>
          <TableBody
            sx={{
              "& .MuiTableRow-root:first-of-type .MuiTableCell-root": {
                fontWeight: 600,
                backgroundColor: theme.palette.backgroundSubtle,
              },
            }}
          >
            <TableRow>
              <TableCell sx={{ width: "30%" }}>Address</TableCell>
              <TableCell sx={{ width: "35%" }}>
                <Tooltip
                  title="The numbers phones use to reach this service center, set as the SMSC number on their SIM"
                  arrow
                >
                  <span>Numbers</span>
                </Tooltip>
              </TableCell>
              <TableCell>Status</TableCell>
              <TableCell align="right" sx={{ width: 100 }}>
                Actions
              </TableCell>
            </TableRow>
            {peersLoading && (
              <TableRow>
                <TableCell colSpan={4}>
                  <Skeleton variant="text" width={240} />
                </TableCell>
              </TableRow>
            )}
            {!peersLoading && peers.length === 0 && (
              <TableRow>
                <TableCell colSpan={4}>
                  <Typography variant="body2" color="textSecondary">
                    No service centers yet.
                  </Typography>
                </TableCell>
              </TableRow>
            )}
            {peers.map((peer) => (
              <TableRow key={peer.id}>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {formatEndpoint(peer.address, peer.port)}
                </TableCell>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {peer.serviceCentres.join(", ")}
                </TableCell>
                <TableCell>
                  <SMSCStatus peer={peer} />
                </TableCell>
                <TableCell align="right">
                  {canEdit && (
                    <>
                      <Tooltip title="Edit service center" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setPeerModal({ peer })}
                          aria-label={`Edit service center ${formatEndpoint(peer.address, peer.port)}`}
                        >
                          <EditIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Delete service center" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setPeerToDelete(peer)}
                          aria-label={`Delete service center ${formatEndpoint(peer.address, peer.port)}`}
                        >
                          <DeleteIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                    </>
                  )}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>

      {numberModalOpen && (
        <EditOperatorSMSModal
          open
          onClose={() => setNumberModalOpen(false)}
          onSuccess={() => {
            refresh();
            showSnackbar("SMS number updated successfully.", "success");
          }}
          initialData={{ smsNumber: sms?.smsNumber ?? "" }}
        />
      )}
      {peerModal && (
        <SMSCPeerModal
          open
          onClose={() => setPeerModal(null)}
          onSuccess={() => {
            refresh();
            showSnackbar(
              peerModal.peer
                ? "Service center updated successfully."
                : "Service center added successfully.",
              "success",
            );
          }}
          peer={peerModal.peer}
        />
      )}
      {peerToDelete && (
        <DeleteConfirmationModal
          open
          onClose={() => setPeerToDelete(null)}
          onConfirm={handleDelete}
          title="Delete Service Center"
          description={
            peers.length === 1 && ready
              ? `Delete the service center ${formatEndpoint(peerToDelete.address, peerToDelete.port)}? It is your only one: subscribers lose SMS until you add another.`
              : `Delete the service center ${formatEndpoint(peerToDelete.address, peerToDelete.port)}? Messages sent to ${peerToDelete.serviceCentres.join(", ")} will be rejected.`
          }
        />
      )}
    </Box>
  );
};

export default SMSSection;
