import React, { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import {
  Alert,
  Box,
  IconButton,
  Typography,
  Chip,
  Tooltip,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableRow,
  Button,
  Skeleton,
  Stack,
  Tabs,
  Tab,
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
import EmptyState from "@/components/EmptyState";
import { useAuth } from "@/contexts/AuthContext";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const isSdSet = (sd?: string | null) =>
  typeof sd === "string" && sd.trim() !== "";

const formatSd = (sd?: string | null) => {
  if (!isSdSet(sd)) return "Not set";
  const v = sd!.startsWith("0x") ? sd! : `0x${sd}`;
  return v.toLowerCase();
};

const profileDescriptions: Record<string, string> = {
  A: "ECIES with X25519 (Curve25519)",
  B: "ECIES with compact P-256 (secp256r1)",
};

const algTooltips: Record<string, string> = {
  NEA0: "5G null ciphering (no encryption)",
  NEA1: "5G NAS encryption with SNOW 3G",
  NEA2: "5G NAS encryption with AES-CTR",
  NEA3: "5G NAS encryption with ZUC",
  NIA0: "5G null integrity (no protection)",
  NIA1: "5G NAS integrity with SNOW 3G",
  NIA2: "5G NAS integrity with AES-CMAC",
  NIA3: "5G NAS integrity with ZUC",
};

const tableContainerSx = {
  border: 1,
  borderColor: "divider",
  borderRadius: 1,
} as const;

type TabKey = "general" | "subscriber-security";

const Operator = () => {
  const theme = useTheme();
  const { role, accessToken, authReady } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();

  const initialTab = (searchParams.get("tab") as TabKey) || "general";
  const [tab, setTab] = useState<TabKey>(initialTab);

  const handleTabChange = (_: React.SyntheticEvent, newValue: TabKey) => {
    setTab(newValue);
    setSearchParams({ tab: newValue }, { replace: true });
  };

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
  const isLoading = operatorQuery.isLoading && !operator;

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

  const valueCellSx = { width: "55%" } as const;
  const settingCellSx = { fontWeight: 600, width: "35%" } as const;
  const actionCellSx = { width: "10%", textAlign: "right" } as const;

  return (
    <Box sx={{ py: 4, px: PAGE_PADDING_X, maxWidth: MAX_WIDTH, mx: "auto" }}>
      <Typography variant="h4" sx={{ mb: 1 }}>
        Operator
      </Typography>

      <Typography variant="body1" color="text.secondary" sx={{ mb: 3 }}>
        Review and configure your operator identifiers and core settings.
      </Typography>

      {operatorQuery.isError && (
        <Alert severity="error" sx={{ mb: 3 }}>
          Failed to load operator configuration.
        </Alert>
      )}

      <Tabs
        value={tab}
        onChange={handleTabChange}
        aria-label="Operator sections"
        sx={{ borderBottom: 1, borderColor: "divider" }}
      >
        <Tab value="general" label="General" />
        <Tab value="subscriber-security" label="Subscriber Security" />
      </Tabs>

      {/* ═══════════════ General Tab ═══════════════ */}
      {tab === "general" && (
        <Box sx={{ mt: 3 }}>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            Operator identity, tracking areas, and network slice configuration.
          </Typography>
          <TableContainer sx={tableContainerSx}>
            <Table>
              <TableBody>
                <TableRow>
                  <TableCell sx={settingCellSx}>
                    <Tooltip
                      title="Mobile Country Code / Mobile Network Code — uniquely identifies your network"
                      arrow
                    >
                      <span>Operator ID (MCC/MNC)</span>
                    </Tooltip>
                  </TableCell>
                  <TableCell sx={valueCellSx}>
                    {isLoading ? (
                      <Skeleton variant="text" width={100} />
                    ) : operator ? (
                      `${operator.id.mcc} / ${operator.id.mnc}`
                    ) : (
                      "N/A"
                    )}
                  </TableCell>
                  <TableCell sx={actionCellSx}>
                    {canEdit && (
                      <Tooltip title="Edit operator identity" arrow>
                        <IconButton
                          size="small"
                          onClick={handleEditOperatorIdClick}
                          aria-label="Edit operator identity"
                        >
                          <EditIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                    )}
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell sx={settingCellSx}>
                    <Tooltip
                      title="Operator variant algorithm configuration key — used for authentication"
                      arrow
                    >
                      <span>Operator Code (OP)</span>
                    </Tooltip>
                  </TableCell>
                  <TableCell sx={valueCellSx}>
                    {isLoading ? (
                      <Skeleton variant="text" width={140} />
                    ) : (
                      "***************"
                    )}
                  </TableCell>
                  <TableCell sx={actionCellSx}>
                    {canEdit && (
                      <Tooltip title="Edit operator code" arrow>
                        <IconButton
                          size="small"
                          onClick={handleEditOperatorCodeClick}
                          aria-label="Edit operator code"
                        >
                          <EditIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                    )}
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell sx={settingCellSx}>
                    <Tooltip
                      title="Tracking Area Codes — used by UEs to register in specific geographic areas"
                      arrow
                    >
                      <span>Supported TACs</span>
                    </Tooltip>
                  </TableCell>
                  <TableCell sx={valueCellSx}>
                    {isLoading ? (
                      <Skeleton variant="text" width={160} />
                    ) : (
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
                    )}
                  </TableCell>
                  <TableCell sx={actionCellSx}>
                    {canEdit && (
                      <Tooltip title="Edit supported TACs" arrow>
                        <IconButton
                          size="small"
                          onClick={handleEditOperatorTrackingClick}
                          aria-label="Edit supported TACs"
                        >
                          <EditIcon fontSize="small" color="primary" />
                        </IconButton>
                      </Tooltip>
                    )}
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell sx={settingCellSx}>
                    <Tooltip
                      title="Slice/Service Type and Slice Differentiator — identifies the network slice"
                      arrow
                    >
                      <span>Slice (SST/SD)</span>
                    </Tooltip>
                  </TableCell>
                  <TableCell sx={valueCellSx}>
                    {isLoading ? (
                      <Skeleton variant="text" width={80} />
                    ) : operator ? (
                      `${operator.slice.sst}${isSdSet(operator.slice.sd) ? ` / ${formatSd(operator.slice.sd)}` : ""}`
                    ) : (
                      "N/A"
                    )}
                  </TableCell>
                  <TableCell sx={actionCellSx}>
                    {canEdit && (
                      <Tooltip title="Edit network slice" arrow>
                        <IconButton
                          size="small"
                          onClick={handleEditOperatorSliceClick}
                          aria-label="Edit network slice"
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
        </Box>
      )}

      {/* ═══════════════ Subscriber Security Tab ═══════════════ */}
      {tab === "subscriber-security" && (
        <Box sx={{ mt: 3 }}>
          {/* ─── NAS Security ─── */}
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            sx={{ mb: 2 }}
          >
            <Typography variant="body2" color="text.secondary">
              Algorithm preference order for NAS ciphering and integrity
              protection.
            </Typography>
            {canEdit && (
              <Tooltip title="Edit NAS security algorithms" arrow>
                <IconButton
                  size="small"
                  onClick={handleEditOperatorSecurityClick}
                  aria-label="Edit NAS security algorithms"
                >
                  <EditIcon fontSize="small" color="primary" />
                </IconButton>
              </Tooltip>
            )}
          </Stack>
          <TableContainer sx={{ ...tableContainerSx, mb: 4 }}>
            <Table>
              <TableBody>
                <TableRow>
                  <TableCell sx={settingCellSx}>Ciphering Order</TableCell>
                  <TableCell sx={valueCellSx}>
                    {isLoading ? (
                      <Skeleton variant="text" width={200} />
                    ) : (
                      <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
                        {operator?.security?.cipheringOrder?.length
                          ? operator.security.cipheringOrder.map((alg, idx) => (
                              <Tooltip
                                key={idx}
                                title={algTooltips[alg] ?? alg}
                                arrow
                              >
                                <Chip
                                  label={`${idx + 1}. ${alg}`}
                                  variant="outlined"
                                  color={alg === "NEA0" ? "warning" : "primary"}
                                  size="small"
                                />
                              </Tooltip>
                            ))
                          : "N/A"}
                      </Box>
                    )}
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell sx={settingCellSx}>Integrity Order</TableCell>
                  <TableCell sx={valueCellSx}>
                    {isLoading ? (
                      <Skeleton variant="text" width={200} />
                    ) : (
                      <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
                        {operator?.security?.integrityOrder?.length
                          ? operator.security.integrityOrder.map((alg, idx) => (
                              <Tooltip
                                key={idx}
                                title={algTooltips[alg] ?? alg}
                                arrow
                              >
                                <Chip
                                  label={`${idx + 1}. ${alg}`}
                                  variant="outlined"
                                  color={alg === "NIA0" ? "warning" : "primary"}
                                  size="small"
                                />
                              </Tooltip>
                            ))
                          : "N/A"}
                      </Box>
                    )}
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </TableContainer>

          {/* ─── Home Network Keys ─── */}
          <Stack
            direction="row"
            alignItems="center"
            justifyContent="space-between"
            sx={{ mb: 2 }}
          >
            <Typography variant="body2" color="text.secondary">
              Keys used for SUCI de-concealment. At least one key is required
              for UE registration with concealed identities.
            </Typography>
            {canEdit && (operator?.homeNetwork.keys.length ?? 0) > 0 && (
              <Button
                variant="contained"
                color="success"
                size="small"
                startIcon={<AddIcon />}
                onClick={() => setCreateHomeNetworkKeyModalOpen(true)}
                aria-label="Add home network key"
                sx={{ ml: 2, flexShrink: 0 }}
              >
                Add Key
              </Button>
            )}
          </Stack>

          {!isLoading && operator?.homeNetwork.keys.length === 0 && (
            <EmptyState
              primaryText="No Home Network Keys"
              secondaryText="SUCI de-concealment is disabled."
              extraContent="UEs attempting to register with a concealed identity (SUCI) will be rejected. Add at least one home network key to restore normal operation."
              button={canEdit}
              buttonText="Add Key"
              onCreate={() => setCreateHomeNetworkKeyModalOpen(true)}
              readOnlyHint={
                canEdit
                  ? undefined
                  : "An admin or network manager must add a key."
              }
            />
          )}

          {isLoading && (
            <TableContainer sx={tableContainerSx}>
              <Table>
                <TableBody>
                  {[1, 2].map((i) => (
                    <TableRow key={i}>
                      <TableCell>
                        <Skeleton variant="text" width={30} />
                      </TableCell>
                      <TableCell>
                        <Skeleton variant="text" width={80} />
                      </TableCell>
                      <TableCell>
                        <Skeleton variant="text" width={300} />
                      </TableCell>
                      <TableCell align="right">
                        <Skeleton variant="circular" width={24} height={24} />
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}

          {(operator?.homeNetwork.keys.length ?? 0) > 0 && (
            <TableContainer sx={tableContainerSx}>
              <Table>
                <TableBody
                  sx={{
                    "& .MuiTableRow-root:first-of-type .MuiTableCell-root": {
                      fontWeight: 600,
                      backgroundColor: theme.palette.backgroundSubtle,
                    },
                  }}
                >
                  <TableRow>
                    <TableCell>ID</TableCell>
                    <TableCell>Profile</TableCell>
                    <TableCell>Public Key</TableCell>
                    <TableCell align="right">Actions</TableCell>
                  </TableRow>
                  {operator?.homeNetwork.keys.map((key) => (
                    <TableRow key={key.id}>
                      <TableCell>
                        <Typography variant="body2">
                          {key.keyIdentifier}
                        </Typography>
                      </TableCell>
                      <TableCell>
                        <Tooltip
                          title={
                            profileDescriptions[key.scheme] ??
                            `Profile ${key.scheme}`
                          }
                          arrow
                        >
                          <Chip
                            label={`Profile ${key.scheme}`}
                            variant="outlined"
                            color="primary"
                            size="small"
                          />
                        </Tooltip>
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
                        <Tooltip title="Copy public key" arrow>
                          <IconButton
                            size="small"
                            onClick={() => handleCopyPublicKey(key.publicKey)}
                            aria-label="Copy public key"
                          >
                            <CopyIcon fontSize="small" color="primary" />
                          </IconButton>
                        </Tooltip>
                        {canEdit && (
                          <Tooltip title="Delete key" arrow>
                            <IconButton
                              size="small"
                              onClick={() => handleDeleteKeyClick(key.id)}
                              aria-label={`Delete key ${key.keyIdentifier}`}
                            >
                              <DeleteIcon fontSize="small" color="primary" />
                            </IconButton>
                          </Tooltip>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          )}
        </Box>
      )}

      {/* ═══════════════ Modals ═══════════════ */}
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
