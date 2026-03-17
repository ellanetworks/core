import React, { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Alert,
  Box,
  CircularProgress,
  IconButton,
  Typography,
  Chip,
  Tooltip,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Button,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import {
  ContentCopy as CopyIcon,
  Edit as EditIcon,
  Add as AddIcon,
  Delete as DeleteIcon,
} from "@mui/icons-material";
import {
  getOperator,
  deleteHomeNetworkKey,
  type OperatorData,
} from "@/queries/operator";
import EditOperatorIdModal from "@/components/EditOperatorIdModal";
import EditOperatorCodeModal from "@/components/EditOperatorCodeModal";
import EditOperatorTrackingModal from "@/components/EditOperatorTrackingModal";
import EditOperatorSliceModal from "@/components/EditOperatorSliceModal";
import CreateHomeNetworkKeyModal from "@/components/CreateHomeNetworkKeyModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EditOperatorSecurityModal from "@/components/EditOperatorSecurityModal";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";

const isSdSet = (sd?: string | null) =>
  typeof sd === "string" && sd.trim() !== "";

const formatSd = (sd?: string | null) => {
  if (!isSdSet(sd)) return "Not set";
  const v = sd!.startsWith("0x") ? sd! : `0x${sd}`;
  return v.toLowerCase();
};

import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const Operator = () => {
  const theme = useTheme();
  const { role, accessToken, authReady } = useAuth();

  const [isEditOperatorIdModalOpen, setEditOperatorIdModalOpen] =
    useState(false);
  const [isEditOperatorCodeModalOpen, setEditOperatorCodeModalOpen] =
    useState(false);
  const [isEditOperatorTrackingModalOpen, setEditOperatorTrackingModalOpen] =
    useState(false);
  const [isEditOperatorSliceModalOpen, setEditOperatorSliceModalOpen] =
    useState(false);
  const [isCreateHomeNetworkKeyModalOpen, setCreateHomeNetworkKeyModalOpen] =
    useState(false);
  const [isDeleteKeyConfirmOpen, setDeleteKeyConfirmOpen] = useState(false);
  const [selectedKeyId, setSelectedKeyId] = useState<number | null>(null);
  const [isEditOperatorSecurityModalOpen, setEditOperatorSecurityModalOpen] =
    useState(false);

  const anyModalOpen =
    isEditOperatorIdModalOpen ||
    isEditOperatorCodeModalOpen ||
    isEditOperatorTrackingModalOpen ||
    isEditOperatorSliceModalOpen ||
    isCreateHomeNetworkKeyModalOpen ||
    isDeleteKeyConfirmOpen ||
    isEditOperatorSecurityModalOpen;

  const queryClient = useQueryClient();
  const operatorQuery = useQuery<OperatorData>({
    queryKey: ["operator"],
    enabled: authReady && !!accessToken && !anyModalOpen,
    queryFn: () => getOperator(accessToken!),
    placeholderData: (prev) => prev,
  });
  const operator = operatorQuery.data ?? null;

  const { showSnackbar } = useSnackbar();

  const canEdit = role === "Admin" || role === "Network Manager";

  const handleEditOperatorIdClick = () => setEditOperatorIdModalOpen(true);
  const handleEditOperatorCodeClick = () => setEditOperatorCodeModalOpen(true);
  const handleEditOperatorTrackingClick = () =>
    setEditOperatorTrackingModalOpen(true);
  const handleEditOperatorSliceClick = () =>
    setEditOperatorSliceModalOpen(true);
  const handleEditOperatorSecurityClick = () =>
    setEditOperatorSecurityModalOpen(true);

  const handleEditOperatorIdModalClose = () =>
    setEditOperatorIdModalOpen(false);
  const handleEditOperatorCodeModalClose = () =>
    setEditOperatorCodeModalOpen(false);
  const handleEditOperatorTrackingModalClose = () =>
    setEditOperatorTrackingModalOpen(false);
  const handleEditOperatorSliceModalClose = () =>
    setEditOperatorSliceModalOpen(false);
  const handleEditOperatorSecurityModalClose = () =>
    setEditOperatorSecurityModalOpen(false);

  const handleEditOperatorIdSuccess = () => {
    queryClient.invalidateQueries({ queryKey: ["operator"] });
    showSnackbar("Operator ID updated successfully.", "success");
  };
  const handleEditOperatorCodeSuccess = () => {
    showSnackbar("Operator Code updated successfully.", "success");
  };
  const handleEditOperatorTrackingSuccess = () => {
    queryClient.invalidateQueries({ queryKey: ["operator"] });
    showSnackbar(
      "Operator Tracking information updated successfully.",
      "success",
    );
  };
  const handleEditOperatorSliceSuccess = () => {
    queryClient.invalidateQueries({ queryKey: ["operator"] });
    showSnackbar("Operator Slice information updated successfully.", "success");
  };
  const handleCreateHomeNetworkKeySuccess = () => {
    queryClient.invalidateQueries({ queryKey: ["operator"] });
    showSnackbar("Home network key created successfully.", "success");
  };
  const handleDeleteKeyClick = (id: number) => {
    setSelectedKeyId(id);
    setDeleteKeyConfirmOpen(true);
  };
  const handleDeleteKeyConfirm = async () => {
    if (selectedKeyId === null || !accessToken) return;
    try {
      await deleteHomeNetworkKey(accessToken, selectedKeyId);
      setDeleteKeyConfirmOpen(false);
      queryClient.invalidateQueries({ queryKey: ["operator"] });
      showSnackbar("Home network key deleted successfully.", "success");
    } catch (error) {
      setDeleteKeyConfirmOpen(false);
      const msg =
        error instanceof Error ? error.message : "Unknown error occurred.";
      showSnackbar(`Failed to delete home network key: ${msg}`, "error");
    } finally {
      setSelectedKeyId(null);
    }
  };
  const handleEditOperatorSecuritySuccess = () => {
    queryClient.invalidateQueries({ queryKey: ["operator"] });
    showSnackbar("NAS security algorithms updated successfully.", "success");
  };

  const handleCopyPublicKey = async (publicKey: string) => {
    if (!navigator.clipboard) {
      showSnackbar(
        "Clipboard API not available. Please use HTTPS or try a different browser.",
        "error",
      );
      return;
    }
    try {
      await navigator.clipboard.writeText(publicKey);
      showSnackbar("Copied to clipboard.", "success");
    } catch {
      showSnackbar("Failed to copy public key.", "error");
    }
  };

  const descriptionText =
    "Review and configure your operator identifiers and core settings.";

  return (
    <Box sx={{ py: 4, px: PAGE_PADDING_X, maxWidth: MAX_WIDTH, mx: "auto" }}>
      <Typography variant="h4" sx={{ mb: 1 }}>
        Operator
      </Typography>

      <Typography variant="body1" color="text.secondary" sx={{ mb: 3 }}>
        {descriptionText}
      </Typography>

      {operatorQuery.isLoading && !operator && (
        <Box sx={{ display: "flex", justifyContent: "center", py: 6 }}>
          <CircularProgress />
        </Box>
      )}

      {operatorQuery.isError && (
        <Alert severity="error" sx={{ mb: 3 }}>
          Failed to load operator configuration.
        </Alert>
      )}

      <TableContainer
        sx={{
          border: 1,
          borderColor: "divider",
          borderRadius: 1,
          mb: 4,
        }}
      >
        <Table>
          <TableHead
            sx={{
              backgroundColor: theme.palette.backgroundSubtle,
              "& .MuiTableCell-head": { fontWeight: 600 },
            }}
          >
            <TableRow>
              <TableCell>Setting</TableCell>
              <TableCell>Value</TableCell>
              <TableCell align="right">Actions</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            <TableRow>
              <TableCell>Operator ID (MCC/MNC)</TableCell>
              <TableCell>
                {operator ? `${operator.id.mcc} / ${operator.id.mnc}` : "N/A"}
              </TableCell>
              <TableCell align="right">
                {canEdit && (
                  <IconButton size="small" onClick={handleEditOperatorIdClick}>
                    <EditIcon fontSize="small" color="primary" />
                  </IconButton>
                )}
              </TableCell>
            </TableRow>
            <TableRow>
              <TableCell>Operator Code (OP)</TableCell>
              <TableCell>{"***************"}</TableCell>
              <TableCell align="right">
                {canEdit && (
                  <IconButton
                    size="small"
                    onClick={handleEditOperatorCodeClick}
                  >
                    <EditIcon fontSize="small" color="primary" />
                  </IconButton>
                )}
              </TableCell>
            </TableRow>
            <TableRow>
              <TableCell>Supported TACs</TableCell>
              <TableCell>
                <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
                  {operator?.tracking.supportedTacs?.length
                    ? operator.tracking.supportedTacs.map((tac, idx) => (
                        <Chip
                          key={idx}
                          label={tac}
                          variant="outlined"
                          color="primary"
                          size="small"
                        />
                      ))
                    : "N/A"}
                </Box>
              </TableCell>
              <TableCell align="right">
                {canEdit && (
                  <IconButton
                    size="small"
                    onClick={handleEditOperatorTrackingClick}
                  >
                    <EditIcon fontSize="small" color="primary" />
                  </IconButton>
                )}
              </TableCell>
            </TableRow>
            <TableRow>
              <TableCell>Slice (SST/SD)</TableCell>
              <TableCell>
                {operator
                  ? `${operator.slice.sst}${isSdSet(operator.slice.sd) ? ` / ${formatSd(operator.slice.sd)}` : ""}`
                  : "N/A"}
              </TableCell>
              <TableCell align="right">
                {canEdit && (
                  <IconButton
                    size="small"
                    onClick={handleEditOperatorSliceClick}
                  >
                    <EditIcon fontSize="small" color="primary" />
                  </IconButton>
                )}
              </TableCell>
            </TableRow>
            <TableRow>
              <TableCell>NAS Security</TableCell>
              <TableCell>
                <Box sx={{ mb: 1 }}>
                  <Typography variant="caption" color="text.secondary">
                    Ciphering
                  </Typography>
                  <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
                    {operator?.security?.cipheringOrder?.length
                      ? operator.security.cipheringOrder.map((alg, idx) => (
                          <Chip
                            key={idx}
                            label={`${idx + 1}. ${alg}`}
                            variant="outlined"
                            color={alg === "NEA0" ? "warning" : "primary"}
                            size="small"
                          />
                        ))
                      : "N/A"}
                  </Box>
                </Box>
                <Box>
                  <Typography variant="caption" color="text.secondary">
                    Integrity
                  </Typography>
                  <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
                    {operator?.security?.integrityOrder?.length
                      ? operator.security.integrityOrder.map((alg, idx) => (
                          <Chip
                            key={idx}
                            label={`${idx + 1}. ${alg}`}
                            variant="outlined"
                            color={alg === "NIA0" ? "warning" : "primary"}
                            size="small"
                          />
                        ))
                      : "N/A"}
                  </Box>
                </Box>
              </TableCell>
              <TableCell align="right">
                {canEdit && (
                  <IconButton
                    size="small"
                    onClick={handleEditOperatorSecurityClick}
                  >
                    <EditIcon fontSize="small" color="primary" />
                  </IconButton>
                )}
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </TableContainer>

      <Box sx={{ mb: 2 }}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            mb: 1,
          }}
        >
          <Typography variant="h5">Home Network Keys</Typography>
          {canEdit && (
            <Button
              variant="contained"
              color="success"
              size="small"
              startIcon={<AddIcon />}
              onClick={() => setCreateHomeNetworkKeyModalOpen(true)}
            >
              Add Key
            </Button>
          )}
        </Box>
      </Box>

      {operator?.homeNetwork.keys.length === 0 && (
        <Alert severity="warning" sx={{ mb: 3 }}>
          No home network keys configured. SUCI de-concealment is disabled. UEs
          attempting to register with a concealed identity (SUCI) will be
          rejected. Add at least one home network key to restore normal
          operation.
        </Alert>
      )}
      {(operator?.homeNetwork.keys.length ?? 0) > 0 && (
        <TableContainer
          sx={{
            border: 1,
            borderColor: "divider",
            borderRadius: 1,
          }}
        >
          <Table size="small">
            <TableHead
              sx={{
                backgroundColor: theme.palette.backgroundSubtle,
                "& .MuiTableCell-head": { fontWeight: 600 },
              }}
            >
              <TableRow>
                <TableCell>ID</TableCell>
                <TableCell>Profile</TableCell>
                <TableCell>Public Key</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {operator?.homeNetwork.keys.map((key) => (
                <TableRow key={key.id}>
                  <TableCell>
                    <Typography variant="body2">{key.keyIdentifier}</Typography>
                  </TableCell>
                  <TableCell>
                    <Chip
                      label={`Profile ${key.scheme}`}
                      variant="outlined"
                      color="primary"
                      size="small"
                    />
                  </TableCell>
                  <TableCell>
                    <Tooltip title={key.publicKey} arrow>
                      <Typography
                        variant="body2"
                        sx={{
                          fontFamily: "monospace",
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                          maxWidth: 400,
                        }}
                      >
                        {key.publicKey}
                      </Typography>
                    </Tooltip>
                  </TableCell>
                  <TableCell align="right">
                    <IconButton
                      size="small"
                      onClick={() => handleCopyPublicKey(key.publicKey)}
                    >
                      <CopyIcon fontSize="small" color="primary" />
                    </IconButton>
                    {canEdit && (
                      <IconButton
                        size="small"
                        onClick={() => handleDeleteKeyClick(key.id)}
                      >
                        <DeleteIcon fontSize="small" color="primary" />
                      </IconButton>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}

      {isEditOperatorIdModalOpen && (
        <EditOperatorIdModal
          open
          onClose={handleEditOperatorIdModalClose}
          onSuccess={handleEditOperatorIdSuccess}
          initialData={operator?.id || { mcc: "", mnc: "" }}
        />
      )}
      {isEditOperatorCodeModalOpen && (
        <EditOperatorCodeModal
          open
          onClose={handleEditOperatorCodeModalClose}
          onSuccess={handleEditOperatorCodeSuccess}
        />
      )}
      {isEditOperatorTrackingModalOpen && (
        <EditOperatorTrackingModal
          open
          onClose={handleEditOperatorTrackingModalClose}
          onSuccess={handleEditOperatorTrackingSuccess}
          initialData={operator?.tracking || { supportedTacs: [""] }}
        />
      )}
      {isEditOperatorSliceModalOpen && (
        <EditOperatorSliceModal
          open
          onClose={handleEditOperatorSliceModalClose}
          onSuccess={handleEditOperatorSliceSuccess}
          initialData={{
            sst: operator?.slice.sst ?? 1,
            sd: operator?.slice.sd ?? "",
          }}
        />
      )}
      {isCreateHomeNetworkKeyModalOpen && (
        <CreateHomeNetworkKeyModal
          open
          onClose={() => setCreateHomeNetworkKeyModalOpen(false)}
          onSuccess={handleCreateHomeNetworkKeySuccess}
        />
      )}
      {isDeleteKeyConfirmOpen &&
        (() => {
          const selectedKey = operator?.homeNetwork.keys.find(
            (k) => k.id === selectedKeyId,
          );
          const keyLabel = selectedKey
            ? `Profile ${selectedKey.scheme}, ID ${selectedKey.keyIdentifier}`
            : "this home network key";
          return (
            <DeleteConfirmationModal
              open
              onClose={() => {
                setDeleteKeyConfirmOpen(false);
                setSelectedKeyId(null);
              }}
              onConfirm={handleDeleteKeyConfirm}
              title="Delete Home Network Key"
              description={`Are you sure you want to delete ${keyLabel}? UEs provisioned with this key will no longer be able to register.`}
            />
          );
        })()}
      {isEditOperatorSecurityModalOpen && (
        <EditOperatorSecurityModal
          open
          onClose={handleEditOperatorSecurityModalClose}
          onSuccess={handleEditOperatorSecuritySuccess}
          initialData={{
            cipheringOrder: operator?.security?.cipheringOrder ?? [
              "NEA2",
              "NEA1",
              "NEA0",
            ],
            integrityOrder: operator?.security?.integrityOrder ?? [
              "NIA2",
              "NIA1",
              "NIA0",
            ],
          }}
        />
      )}
    </Box>
  );
};

export default Operator;
