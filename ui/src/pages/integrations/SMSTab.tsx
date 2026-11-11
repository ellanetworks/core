// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Box,
  Button,
  Alert,
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
  getSMSSettings,
  listSMSCs,
  type SMSC,
  type SMSSettings,
} from "@/queries/integrations";
import EditServingNodeNumberModal, {
  SERVING_NODE_NUMBER_HELP,
} from "@/components/EditServingNodeNumberModal";
import ConnectionsStatusChip from "@/components/ConnectionsStatusChip";
import SMSCModal from "@/components/SMSCModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import {
  ACTION_CELL_SX,
  SETTING_CELL_SX,
  TABLE_CONTAINER_SX,
  VALUE_CELL_SX,
} from "@/utils/layout";
import { useIntegrationsContext } from "./types";
import { ThisCoreSection, formatEndpoint, useDiameterStatus } from "./diameter";

export default function SMSTab() {
  const { canEdit } = useIntegrationsContext();
  const theme = useTheme();
  const { accessToken, authReady } = useAuth();
  const { showSnackbar } = useSnackbar();
  const queryClient = useQueryClient();

  const [numberModalOpen, setNumberModalOpen] = useState(false);
  const [smscModal, setSMSCModal] = useState<{ smsc?: SMSC } | null>(null);
  const [smscToDelete, setSMSCToDelete] = useState<SMSC | null>(null);

  const anyModalOpen =
    numberModalOpen || smscModal !== null || smscToDelete !== null;

  const diameterQuery = useDiameterStatus(!anyModalOpen);

  const settingsQuery = useQuery<SMSSettings>({
    queryKey: ["integrations-sms"],
    enabled: authReady && !!accessToken && !anyModalOpen,
    queryFn: () => getSMSSettings(accessToken!),
    placeholderData: (prev) => prev,
  });
  const sms = settingsQuery.data;
  const settingsLoading = settingsQuery.isLoading && !sms;

  const smscsQuery = useQuery<SMSC[]>({
    queryKey: ["smscs"],
    enabled: authReady && !!accessToken && !anyModalOpen,
    refetchInterval: 5000,
    queryFn: () => listSMSCs(accessToken!),
    placeholderData: (prev) => prev,
  });

  const smscs = smscsQuery.data ?? [];
  const smscsLoading = smscsQuery.isLoading && !smscsQuery.data;
  const hasNumber = !!sms?.servingNodeNumber;
  const ready = smscs.length > 0 && hasNumber;

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ["integrations-sms"] });
    queryClient.invalidateQueries({ queryKey: ["smscs"] });
    queryClient.invalidateQueries({ queryKey: ["integrations-diameter"] });
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
    <Box sx={{ mt: 3 }}>
      <Typography variant="body2" color="textSecondary" sx={{ mb: 2 }}>
        Let 4G and 5G subscribers send and receive SMS. Each subscriber needs a
        phone number.
      </Typography>

      {settingsQuery.isError && (
        <Alert severity="error" sx={{ mb: 2 }}>
          Failed to load SMS settings.
        </Alert>
      )}
      {sms && !hasNumber && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          No serving node number is set: messages sent to your subscribers
          cannot be delivered. Set it below.
        </Alert>
      )}
      {smscsQuery.data && smscs.length === 0 && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          No SMSC is configured: subscribers cannot send or receive SMS. Add one
          below.
        </Alert>
      )}

      <ThisCoreSection
        diameter={diameterQuery.data}
        loading={diameterQuery.isLoading && !diameterQuery.data}
        description="Ella Core's settings for your SMSCs. Enter its Diameter Realm as the HSS realm on your SMSC, and its Origin-Host if your SMSC asks for it."
        realmHelp="Ella Core's Diameter realm, set by the Operator ID (MCC/MNC). Your SMSC sends S6c requests to it, as its HSS realm."
        after={
          <>
            <TableRow>
              <TableCell sx={SETTING_CELL_SX}>
                <Tooltip title={SERVING_NODE_NUMBER_HELP} arrow>
                  <span>Serving Node Number</span>
                </Tooltip>
              </TableCell>
              <TableCell sx={VALUE_CELL_SX}>
                {settingsLoading ? (
                  <Skeleton variant="text" width={140} />
                ) : (
                  sms?.servingNodeNumber || "N/A"
                )}
              </TableCell>
              <TableCell sx={ACTION_CELL_SX}>
                {canEdit && (
                  <Tooltip title="Edit serving node number" arrow>
                    <IconButton
                      size="small"
                      onClick={() => setNumberModalOpen(true)}
                      aria-label="Edit serving node number"
                    >
                      <EditIcon fontSize="small" color="primary" />
                    </IconButton>
                  </Tooltip>
                )}
              </TableCell>
            </TableRow>
          </>
        }
      />

      <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 2 }}>
        SMSCs
      </Typography>
      <Stack
        direction="row"
        sx={{ alignItems: "center", justifyContent: "space-between", mb: 2 }}
      >
        <Typography variant="body2" color="textSecondary">
          Service centres that store and deliver your subscribers&apos;
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
                  <span>Service Centre Addresses</span>
                </Tooltip>
              </TableCell>
              <TableCell>Diameter Peers</TableCell>
              <TableCell sx={{ width: 170 }}>Status</TableCell>
              <TableCell align="right" sx={{ width: 100 }}>
                Actions
              </TableCell>
            </TableRow>
            {smscsLoading && (
              <TableRow>
                <TableCell colSpan={5}>
                  <Skeleton variant="text" width={240} />
                </TableCell>
              </TableRow>
            )}
            {!smscsLoading && smscs.length === 0 && (
              <TableRow>
                <TableCell colSpan={5}>
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
                  <Stack spacing={0.5}>
                    {smsc.nodes.map((node) => (
                      <Tooltip
                        key={formatEndpoint(node.address, node.port)}
                        title={`Origin-Host: ${node.originHost || "any"}`}
                        arrow
                      >
                        <Typography
                          variant="body2"
                          sx={{ wordBreak: "break-word", width: "fit-content" }}
                        >
                          {formatEndpoint(node.address, node.port)}
                        </Typography>
                      </Tooltip>
                    ))}
                  </Stack>
                </TableCell>
                <TableCell>
                  <ConnectionsStatusChip
                    connections={smsc.nodes.map((node) => ({
                      label: formatEndpoint(node.address, node.port),
                      status: node.status,
                    }))}
                  />
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
        <EditServingNodeNumberModal
          open
          onClose={() => setNumberModalOpen(false)}
          onSuccess={() => {
            refresh();
            showSnackbar(
              "Serving node number updated successfully.",
              "success",
            );
          }}
          initialData={{ servingNodeNumber: sms?.servingNodeNumber ?? "" }}
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
}
