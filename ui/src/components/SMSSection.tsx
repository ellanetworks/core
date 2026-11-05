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
  deleteSMSC,
  listSMSCs,
  type OperatorSMS,
  type SMSC,
  type SMSCNode,
  type SMSCNodeState,
} from "@/queries/operator";
import EditOperatorSMSModal from "@/components/EditOperatorSMSModal";
import SMSCModal from "@/components/SMSCModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { TABLE_CONTAINER_SX } from "@/utils/layout";
import { formatDateTime } from "@/utils/formatters";

const stateLabels: Record<
  SMSCNodeState,
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

const NodeStatus: React.FC<{ node: SMSCNode }> = ({ node }) => {
  const status = node.status;
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
  const [smscModal, setSMSCModal] = useState<{ smsc?: SMSC } | null>(null);
  const [smscToDelete, setSMSCToDelete] = useState<SMSC | null>(null);

  const anyModalOpen =
    numberModalOpen || smscModal !== null || smscToDelete !== null;

  useEffect(() => {
    onModalOpenChange(anyModalOpen);
  }, [anyModalOpen, onModalOpenChange]);

  const smscsQuery = useQuery<SMSC[]>({
    queryKey: ["smscs"],
    enabled: authReady && !!accessToken && !anyModalOpen,
    refetchInterval: 5000,
    queryFn: () => listSMSCs(accessToken!),
    placeholderData: (prev) => prev,
  });

  const smscs = smscsQuery.data ?? [];
  const smscsLoading = smscsQuery.isLoading && !smscsQuery.data;
  const hasNumber = !!sms?.smsNumber;
  const ready = smscs.length > 0 && hasNumber;

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ["operator"] });
    queryClient.invalidateQueries({ queryKey: ["smscs"] });
  };

  const handleDelete = async () => {
    if (!smscToDelete || !accessToken) return;
    try {
      await deleteSMSC(accessToken, smscToDelete.name);
      refresh();
      showSnackbar("SMSC deleted successfully.", "success");
    } catch (error) {
      const msg =
        error instanceof Error ? error.message : "Unknown error occurred.";
      showSnackbar(`Failed to delete SMSC: ${msg}`, "error");
    } finally {
      setSMSCToDelete(null);
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
        SMSCs
      </Typography>
      <Stack
        direction="row"
        sx={{ alignItems: "center", justifyContent: "space-between", mb: 2 }}
      >
        <Typography variant="body2" color="textSecondary">
          Service centers that store and deliver your subscribers&apos;
          messages.
        </Typography>
        {canEdit && (
          <Button
            variant="contained"
            color="success"
            size="small"
            startIcon={<AddIcon />}
            onClick={() => setSMSCModal({})}
            aria-label="Add SMSC"
            sx={{ ml: 2, flexShrink: 0 }}
          >
            Add SMSC
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
              <TableCell sx={{ width: "20%" }}>Name</TableCell>
              <TableCell sx={{ width: "25%" }}>
                <Tooltip
                  title="The numbers phones use to reach this SMSC, set as the SMSC number on their SIM"
                  arrow
                >
                  <span>Numbers</span>
                </Tooltip>
              </TableCell>
              <TableCell>Nodes</TableCell>
              <TableCell align="right" sx={{ width: 100 }}>
                Actions
              </TableCell>
            </TableRow>
            {smscsLoading && (
              <TableRow>
                <TableCell colSpan={4}>
                  <Skeleton variant="text" width={240} />
                </TableCell>
              </TableRow>
            )}
            {!smscsLoading && smscs.length === 0 && (
              <TableRow>
                <TableCell colSpan={4}>
                  <Typography variant="body2" color="textSecondary">
                    No SMSCs yet.
                  </Typography>
                </TableCell>
              </TableRow>
            )}
            {smscs.map((smsc) => (
              <TableRow key={smsc.name} sx={{ verticalAlign: "top" }}>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {smsc.name}
                </TableCell>
                <TableCell sx={{ wordBreak: "break-word" }}>
                  {smsc.serviceCentres.join(", ")}
                </TableCell>
                <TableCell>
                  <Box
                    sx={{
                      display: "grid",
                      gridTemplateColumns: "auto 1fr",
                      columnGap: 1.5,
                      rowGap: 1,
                      alignItems: "center",
                      justifyItems: "start",
                    }}
                  >
                    {smsc.nodes.map((node) => (
                      <React.Fragment
                        key={formatEndpoint(node.address, node.port)}
                      >
                        <Tooltip title={node.diameterIdentity} arrow>
                          <Typography
                            variant="body2"
                            sx={{ wordBreak: "break-word" }}
                          >
                            {formatEndpoint(node.address, node.port)}
                          </Typography>
                        </Tooltip>
                        <NodeStatus node={node} />
                      </React.Fragment>
                    ))}
                  </Box>
                </TableCell>
                <TableCell align="right">
                  {canEdit && (
                    <>
                      <Tooltip title="Edit SMSC" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setSMSCModal({ smsc })}
                          aria-label={`Edit SMSC ${smsc.name}`}
                        >
                          <EditIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Delete SMSC" arrow>
                        <IconButton
                          size="small"
                          onClick={() => setSMSCToDelete(smsc)}
                          aria-label={`Delete SMSC ${smsc.name}`}
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
      {smscModal && (
        <SMSCModal
          open
          onClose={() => setSMSCModal(null)}
          onSuccess={() => {
            refresh();
            showSnackbar(
              smscModal.smsc
                ? "SMSC updated successfully."
                : "SMSC added successfully.",
              "success",
            );
          }}
          smsc={smscModal.smsc}
        />
      )}
      {smscToDelete && (
        <DeleteConfirmationModal
          open
          onClose={() => setSMSCToDelete(null)}
          onConfirm={handleDelete}
          title="Delete SMSC"
          description={
            smscs.length === 1 && ready
              ? `Delete the SMSC ${smscToDelete.name}? It is your only one: subscribers lose SMS until you add another.`
              : `Delete the SMSC ${smscToDelete.name}? Messages sent to ${smscToDelete.serviceCentres.join(", ")} will be rejected.`
          }
        />
      )}
    </Box>
  );
};

export default SMSSection;
