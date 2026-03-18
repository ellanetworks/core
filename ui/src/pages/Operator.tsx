import React, { useState, useRef, useEffect } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
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
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import {
  ContentCopy as CopyIcon,
  Edit as EditIcon,
  Add as AddIcon,
  Delete as DeleteIcon,
  Visibility as VisibilityIcon,
  VisibilityOff as VisibilityOffIcon,
} from "@mui/icons-material";
import {
  getOperator,
  deleteHomeNetworkKey,
  getHomeNetworkKeyPrivateKey,
  type OperatorData,
} from "@/queries/operator";
import EditOperatorIdModal from "@/components/EditOperatorIdModal";
import EditOperatorCodeModal from "@/components/EditOperatorCodeModal";
import EditOperatorTrackingModal from "@/components/EditOperatorTrackingModal";
import EditOperatorSliceModal from "@/components/EditOperatorSliceModal";
import CreateHomeNetworkKeyModal from "@/components/CreateHomeNetworkKeyModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EditOperatorNASSecurityModal from "@/components/EditOperatorNASSecurityModal";
import EditOperatorSPNModal from "@/components/EditOperatorSPNModal";
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
  const [
    isEditOperatorNASSecurityModalOpen,
    setEditOperatorNASSecurityModalOpen,
  ] = useState(false);
  const [isEditOperatorSPNModalOpen, setEditOperatorSPNModalOpen] =
    useState(false);
  const [visiblePrivateKeys, setVisiblePrivateKeys] = useState<
    Record<number, string>
  >({});
  const [loadingPrivateKeys, setLoadingPrivateKeys] = useState<
    Record<number, boolean>
  >({});
  const privateKeyTimers = useRef<
    Record<number, ReturnType<typeof setTimeout>>
  >({});

  useEffect(() => {
    const timers = privateKeyTimers.current;
    return () => {
      Object.values(timers).forEach(clearTimeout);
    };
  }, []);

  const anyModalOpen =
    isEditOperatorIdModalOpen ||
    isEditOperatorCodeModalOpen ||
    isEditOperatorTrackingModalOpen ||
    isEditOperatorSliceModalOpen ||
    isCreateHomeNetworkKeyModalOpen ||
    isDeleteKeyConfirmOpen ||
    isEditOperatorNASSecurityModalOpen ||
    isEditOperatorSPNModalOpen;

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
  const handleEditOperatorNASSecurityClick = () =>
    setEditOperatorNASSecurityModalOpen(true);
  const handleEditOperatorSPNClick = () => setEditOperatorSPNModalOpen(true);

  const handleEditOperatorIdModalClose = () =>
    setEditOperatorIdModalOpen(false);
  const handleEditOperatorCodeModalClose = () =>
    setEditOperatorCodeModalOpen(false);
  const handleEditOperatorTrackingModalClose = () =>
    setEditOperatorTrackingModalOpen(false);
  const handleEditOperatorSliceModalClose = () =>
    setEditOperatorSliceModalOpen(false);
  const handleEditOperatorNASSecurityModalClose = () =>
    setEditOperatorNASSecurityModalOpen(false);
  const handleEditOperatorSPNModalClose = () =>
    setEditOperatorSPNModalOpen(false);

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
  const handleEditOperatorNASSecuritySuccess = () => {
    queryClient.invalidateQueries({ queryKey: ["operator"] });
    showSnackbar("NAS security algorithms updated successfully.", "success");
  };
  const handleEditOperatorSPNSuccess = () => {
    queryClient.invalidateQueries({ queryKey: ["operator"] });
    showSnackbar("Network name (SPN) updated successfully.", "success");
  };

  const handleCopyToClipboard = async (publicKey: string) => {
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
      showSnackbar("Failed to copy to clipboard.", "error");
    }
  };

  const clearPrivateKey = (keyId: number) => {
    clearTimeout(privateKeyTimers.current[keyId]);
    delete privateKeyTimers.current[keyId];
    setVisiblePrivateKeys((prev) => {
      const next = { ...prev };
      delete next[keyId];
      return next;
    });
  };

  const handleTogglePrivateKey = async (keyId: number) => {
    if (visiblePrivateKeys[keyId] !== undefined) {
      clearPrivateKey(keyId);
      return;
    }
    if (!accessToken) return;
    setLoadingPrivateKeys((prev) => ({ ...prev, [keyId]: true }));
    try {
      const result = await getHomeNetworkKeyPrivateKey(accessToken, keyId);
      setVisiblePrivateKeys((prev) => ({
        ...prev,
        [keyId]: result.privateKey,
      }));
      // Auto-clear the private key from state after 30 seconds.
      clearTimeout(privateKeyTimers.current[keyId]);
      privateKeyTimers.current[keyId] = setTimeout(() => {
        clearPrivateKey(keyId);
      }, 30_000);
    } catch (error) {
      const msg =
        error instanceof Error ? error.message : "Unknown error occurred.";
      showSnackbar(`Failed to retrieve private key: ${msg}`, "error");
    } finally {
      setLoadingPrivateKeys((prev) => ({ ...prev, [keyId]: false }));
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

      {/* ═══════════════ Configuration ═══════════════ */}
      <Box sx={{ mt: 3 }}>
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
                    title="Service Provider Name — the network name displayed on connected devices"
                    arrow
                  >
                    <span>Network Name (SPN)</span>
                  </Tooltip>
                </TableCell>
                <TableCell sx={valueCellSx}>
                  {isLoading ? (
                    <Skeleton variant="text" width={160} />
                  ) : operator ? (
                    <Table size="small" sx={{ m: -1 }}>
                      <TableBody>
                        <TableRow
                          sx={{ "& td": { borderBottom: "none", py: 0.5 } }}
                        >
                          <TableCell
                            sx={{ fontWeight: 600, width: "30%", pl: 0 }}
                          >
                            Full
                          </TableCell>
                          <TableCell sx={{ pl: 0 }}>
                            {operator.spn?.spnFull || "N/A"}
                          </TableCell>
                        </TableRow>
                        <TableRow
                          sx={{ "& td": { borderBottom: "none", py: 0.5 } }}
                        >
                          <TableCell
                            sx={{ fontWeight: 600, width: "30%", pl: 0 }}
                          >
                            Short
                          </TableCell>
                          <TableCell sx={{ pl: 0 }}>
                            {operator.spn?.spnShort || "N/A"}
                          </TableCell>
                        </TableRow>
                      </TableBody>
                    </Table>
                  ) : (
                    "N/A"
                  )}
                </TableCell>
                <TableCell sx={actionCellSx}>
                  {canEdit && (
                    <Tooltip title="Edit network name (SPN)" arrow>
                      <IconButton
                        size="small"
                        onClick={handleEditOperatorSPNClick}
                        aria-label="Edit network name"
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
              <TableRow>
                <TableCell sx={settingCellSx}>
                  <Tooltip
                    title="Algorithm preference order for NAS ciphering and integrity protection"
                    arrow
                  >
                    <span>NAS Security</span>
                  </Tooltip>
                </TableCell>
                <TableCell sx={valueCellSx}>
                  <Table size="small" sx={{ m: -1 }}>
                    <TableBody>
                      <TableRow
                        sx={{ "& td": { borderBottom: "none", py: 0.5 } }}
                      >
                        <TableCell
                          sx={{ fontWeight: 600, width: "30%", pl: 0 }}
                        >
                          Ciphering
                        </TableCell>
                        <TableCell sx={{ pl: 0 }}>
                          {isLoading ? (
                            <Skeleton variant="text" width={200} />
                          ) : (
                            <Box
                              sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}
                            >
                              {operator?.nasSecurity?.ciphering?.length
                                ? operator.nasSecurity.ciphering.map(
                                    (alg, idx) => (
                                      <Tooltip
                                        key={idx}
                                        title={algTooltips[alg] ?? alg}
                                        arrow
                                      >
                                        <Chip
                                          label={`${idx + 1}. ${alg}`}
                                          variant="outlined"
                                          color={
                                            alg === "NEA0"
                                              ? "warning"
                                              : "primary"
                                          }
                                          size="small"
                                        />
                                      </Tooltip>
                                    ),
                                  )
                                : "N/A"}
                            </Box>
                          )}
                        </TableCell>
                      </TableRow>
                      <TableRow
                        sx={{ "& td": { borderBottom: "none", py: 0.5 } }}
                      >
                        <TableCell
                          sx={{ fontWeight: 600, width: "30%", pl: 0 }}
                        >
                          Integrity
                        </TableCell>
                        <TableCell sx={{ pl: 0 }}>
                          {isLoading ? (
                            <Skeleton variant="text" width={200} />
                          ) : (
                            <Box
                              sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}
                            >
                              {operator?.nasSecurity?.integrity?.length
                                ? operator.nasSecurity.integrity.map(
                                    (alg, idx) => (
                                      <Tooltip
                                        key={idx}
                                        title={algTooltips[alg] ?? alg}
                                        arrow
                                      >
                                        <Chip
                                          label={`${idx + 1}. ${alg}`}
                                          variant="outlined"
                                          color={
                                            alg === "NIA0"
                                              ? "warning"
                                              : "primary"
                                          }
                                          size="small"
                                        />
                                      </Tooltip>
                                    ),
                                  )
                                : "N/A"}
                            </Box>
                          )}
                        </TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </TableCell>
                <TableCell sx={actionCellSx}>
                  {canEdit && (
                    <Tooltip title="Edit NAS security algorithms" arrow>
                      <IconButton
                        size="small"
                        onClick={handleEditOperatorNASSecurityClick}
                        aria-label="Edit NAS security algorithms"
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

      {/* ═══════════════ Home Network Keys ═══════════════ */}
      <Box sx={{ mt: 4 }}>
        <Typography variant="h6" sx={{ mb: 2 }}>
          Home Network Keys
        </Typography>
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          sx={{ mb: 2 }}
        >
          <Typography variant="body2" color="text.secondary">
            Keys used for SUCI de-concealment. At least one key is required for
            UE registration with concealed identities.
          </Typography>
          {canEdit && (
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

        {!isLoading && operator?.homeNetworkKeys.length === 0 && (
          <TableContainer sx={tableContainerSx}>
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
                  <TableCell sx={{ width: 50 }}>ID</TableCell>
                  <TableCell sx={{ width: 110 }}>Profile</TableCell>
                  <TableCell sx={{ width: "30%" }}>Public Key</TableCell>
                  <TableCell sx={{ width: "30%" }}>Private Key</TableCell>
                  <TableCell align="right" sx={{ width: 80 }}>
                    Actions
                  </TableCell>
                </TableRow>
                <TableRow>
                  <TableCell colSpan={5} sx={{ p: 0, borderBottom: "none" }}>
                    <Alert severity="warning" sx={{ borderRadius: 0 }}>
                      No home network keys configured. UEs attempting to
                      register with a concealed identity (SUCI) will be
                      rejected.
                    </Alert>
                  </TableCell>
                </TableRow>
              </TableBody>
            </Table>
          </TableContainer>
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

        {(operator?.homeNetworkKeys.length ?? 0) > 0 && (
          <TableContainer sx={tableContainerSx}>
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
                  <TableCell sx={{ width: 50 }}>ID</TableCell>
                  <TableCell sx={{ width: 110 }}>Profile</TableCell>
                  <TableCell sx={{ width: "30%" }}>Public Key</TableCell>
                  <TableCell sx={{ width: "30%" }}>Private Key</TableCell>
                  <TableCell align="right" sx={{ width: 80 }}>
                    Actions
                  </TableCell>
                </TableRow>
                {operator?.homeNetworkKeys.map((key) => (
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
                      <Box
                        sx={{
                          display: "flex",
                          alignItems: "center",
                          gap: 0.5,
                        }}
                      >
                        <Tooltip title={key.publicKey} arrow>
                          <Typography
                            variant="body2"
                            sx={{
                              fontFamily: "monospace",
                              overflow: "hidden",
                              textOverflow: "ellipsis",
                              whiteSpace: "nowrap",
                              maxWidth: 300,
                            }}
                          >
                            {key.publicKey}
                          </Typography>
                        </Tooltip>
                        <Tooltip title="Copy public key" arrow>
                          <IconButton
                            size="small"
                            onClick={() => handleCopyToClipboard(key.publicKey)}
                            aria-label="Copy public key"
                          >
                            <CopyIcon fontSize="small" color="primary" />
                          </IconButton>
                        </Tooltip>
                      </Box>
                    </TableCell>
                    <TableCell>
                      <Box
                        sx={{
                          display: "flex",
                          alignItems: "center",
                          gap: 0.5,
                        }}
                      >
                        {visiblePrivateKeys[key.id] !== undefined ? (
                          <>
                            <Typography
                              variant="body2"
                              sx={{
                                fontFamily: "monospace",
                                overflow: "hidden",
                                textOverflow: "ellipsis",
                                whiteSpace: "nowrap",
                                flex: 1,
                                minWidth: 0,
                              }}
                            >
                              {visiblePrivateKeys[key.id]}
                            </Typography>
                            <Tooltip title="Copy private key" arrow>
                              <IconButton
                                size="small"
                                onClick={() =>
                                  handleCopyToClipboard(
                                    visiblePrivateKeys[key.id],
                                  )
                                }
                                aria-label="Copy private key"
                              >
                                <CopyIcon fontSize="small" color="primary" />
                              </IconButton>
                            </Tooltip>
                          </>
                        ) : (
                          <Typography
                            variant="body2"
                            sx={{
                              fontFamily: "monospace",
                              overflow: "hidden",
                              textOverflow: "ellipsis",
                              whiteSpace: "nowrap",
                              flex: 1,
                              minWidth: 0,
                            }}
                          >
                            ••••••••••••••••••••••••••••••••••••••
                          </Typography>
                        )}
                        {canEdit && (
                          <Tooltip
                            title={
                              visiblePrivateKeys[key.id] !== undefined
                                ? "Hide private key"
                                : "View private key"
                            }
                            arrow
                          >
                            <IconButton
                              size="small"
                              onClick={() => handleTogglePrivateKey(key.id)}
                              disabled={loadingPrivateKeys[key.id]}
                              aria-label={
                                visiblePrivateKeys[key.id] !== undefined
                                  ? "Hide private key"
                                  : "View private key"
                              }
                            >
                              {visiblePrivateKeys[key.id] !== undefined ? (
                                <VisibilityOffIcon
                                  fontSize="small"
                                  color="primary"
                                />
                              ) : (
                                <VisibilityIcon
                                  fontSize="small"
                                  color="primary"
                                />
                              )}
                            </IconButton>
                          </Tooltip>
                        )}
                      </Box>
                    </TableCell>
                    <TableCell align="right">
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
          const selectedKey = operator?.homeNetworkKeys.find(
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
      {isEditOperatorNASSecurityModalOpen && (
        <EditOperatorNASSecurityModal
          open
          onClose={handleEditOperatorNASSecurityModalClose}
          onSuccess={handleEditOperatorNASSecuritySuccess}
          initialData={{
            ciphering: operator?.nasSecurity?.ciphering ?? [
              "NEA2",
              "NEA1",
              "NEA0",
            ],
            integrity: operator?.nasSecurity?.integrity ?? [
              "NIA2",
              "NIA1",
              "NIA0",
            ],
          }}
        />
      )}
      {isEditOperatorSPNModalOpen && (
        <EditOperatorSPNModal
          open
          onClose={handleEditOperatorSPNModalClose}
          onSuccess={handleEditOperatorSPNSuccess}
          initialData={{
            spnFull: operator?.spn?.spnFull ?? "Ella Core",
            spnShort: operator?.spn?.spnShort ?? "Ella",
          }}
        />
      )}
    </Box>
  );
};

export default Operator;
