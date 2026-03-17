import React, { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Alert,
  Box,
  CircularProgress,
  IconButton,
  Typography,
  Chip,
  Card,
  CardHeader,
  CardContent,
  Tooltip,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Button,
} from "@mui/material";
import Grid from "@mui/material/Grid";
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

  const headerStyles = {
    backgroundColor: "backgroundSubtle",
    color: "text.primary",
    borderTopLeftRadius: 12,
    borderTopRightRadius: 12,
    py: 0.5,
    minHeight: 48,
    alignItems: "center",
    "& .MuiCardHeader-action": {
      marginTop: 0,
      marginBottom: 0,
      alignSelf: "center",
    },
    "& .MuiCardHeader-title": { color: "text.primary", fontSize: "0.95rem" },
    "& .MuiIconButton-root": { color: "text.primary" },
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

      <Grid
        container
        spacing={4}
        justifyContent="flex-start"
        alignItems="stretch"
      >
        <Grid size={{ xs: 12 }}>
          <Typography variant="h5" sx={{ fontWeight: 600 }}>
            Network Identity
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Core identifiers and network slice configuration.
          </Typography>
        </Grid>

        <Grid size={{ xs: 12, sm: 8, md: 6 }}>
          <Card
            sx={{
              height: "100%",
              display: "flex",
              flexDirection: "column",
              borderRadius: 3,
              boxShadow: 2,
            }}
          >
            <CardHeader
              title="Operator ID"
              sx={headerStyles}
              action={
                canEdit && (
                  <IconButton
                    aria-label="edit"
                    onClick={handleEditOperatorIdClick}
                  >
                    <EditIcon color={"primary"} />
                  </IconButton>
                )
              }
            />
            <CardContent>
              <Grid container spacing={1}>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body2" color="text.secondary">
                    Mobile Country Code (MCC)
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body1">
                    {operator?.id.mcc || "N/A"}
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body2" color="text.secondary">
                    Mobile Network Code (MNC)
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body1">
                    {operator?.id.mnc || "N/A"}
                  </Typography>
                </Grid>
              </Grid>
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12, sm: 8, md: 6 }}>
          <Card
            sx={{
              height: "100%",
              display: "flex",
              flexDirection: "column",
              borderRadius: 3,
              boxShadow: 2,
            }}
          >
            <CardHeader
              title="Operator Code"
              sx={headerStyles}
              action={
                canEdit && (
                  <IconButton
                    aria-label="edit"
                    onClick={handleEditOperatorCodeClick}
                  >
                    <EditIcon color={"primary"} />
                  </IconButton>
                )
              }
            />
            <CardContent>
              <Grid container spacing={1}>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body2" color="text.secondary">
                    Operator Code (OP)
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body1">{"***************"}</Typography>
                </Grid>
              </Grid>
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12, sm: 8, md: 6 }}>
          <Card
            sx={{
              height: "100%",
              display: "flex",
              flexDirection: "column",
              borderRadius: 3,
              boxShadow: 2,
            }}
          >
            <CardHeader
              title="Tracking Information"
              sx={headerStyles}
              action={
                canEdit && (
                  <IconButton
                    aria-label="edit"
                    onClick={handleEditOperatorTrackingClick}
                  >
                    <EditIcon color={"primary"} />
                  </IconButton>
                )
              }
            />
            <CardContent>
              <Grid container spacing={1}>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body2" color="text.secondary">
                    Supported TACs
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
                    {operator?.tracking.supportedTacs?.length ? (
                      operator.tracking.supportedTacs.map((tac, idx) => (
                        <Chip
                          key={idx}
                          label={tac}
                          variant="outlined"
                          color="primary"
                        />
                      ))
                    ) : (
                      <Typography variant="body1">
                        No TACs available.
                      </Typography>
                    )}
                  </Box>
                </Grid>
              </Grid>
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12, sm: 8, md: 6 }}>
          <Card
            sx={{
              height: "100%",
              display: "flex",
              flexDirection: "column",
              borderRadius: 3,
              boxShadow: 2,
            }}
          >
            <CardHeader
              title="Slice Information"
              sx={headerStyles}
              action={
                canEdit && (
                  <IconButton
                    aria-label="edit"
                    onClick={handleEditOperatorSliceClick}
                  >
                    <EditIcon color={"primary"} />
                  </IconButton>
                )
              }
            />
            <CardContent>
              <Grid container spacing={1}>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body2" color="text.secondary">
                    Slice/Service Type (SST)
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body1">
                    {operator ? `${operator.slice.sst}` : "N/A"}
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body2" color="text.secondary">
                    Slice Differentiator (SD)
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  {operator ? (
                    isSdSet(operator.slice.sd) ? (
                      <Chip
                        label={formatSd(operator.slice.sd)}
                        color="primary"
                        variant="outlined"
                      />
                    ) : (
                      <Typography variant="body1">N/A</Typography>
                    )
                  ) : (
                    <Typography variant="body1">N/A</Typography>
                  )}
                </Grid>
              </Grid>
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12 }}>
          <Typography variant="h5" sx={{ fontWeight: 600 }}>
            Subscriber Security
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Encryption keys and NAS security algorithms for subscriber
            authentication.
          </Typography>
        </Grid>

        <Grid size={{ xs: 12 }}>
          <Card
            sx={{
              display: "flex",
              flexDirection: "column",
              borderRadius: 3,
              boxShadow: 2,
            }}
          >
            <CardHeader
              title="Home Network Keys"
              sx={headerStyles}
              action={
                canEdit && (
                  <Button
                    variant="contained"
                    color="success"
                    size="small"
                    startIcon={<AddIcon />}
                    onClick={() => setCreateHomeNetworkKeyModalOpen(true)}
                  >
                    Add Key
                  </Button>
                )
              }
            />
            <CardContent sx={{ p: 0, "&:last-child": { pb: 0 } }}>
              {operator?.homeNetwork.keys.length === 0 && (
                <Alert severity="warning" sx={{ m: 2 }}>
                  No home network keys configured. SUCI de-concealment is
                  disabled. UEs attempting to register with a concealed identity
                  (SUCI) will be rejected. Add at least one home network key to
                  restore normal operation.
                </Alert>
              )}
              {(operator?.homeNetwork.keys.length ?? 0) > 0 && (
                <TableContainer>
                  <Table size="small">
                    <TableHead>
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
                            <Typography variant="body2">
                              {key.keyIdentifier}
                            </Typography>
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
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12 }}>
          <Card
            sx={{
              height: "100%",
              display: "flex",
              flexDirection: "column",
              borderRadius: 3,
              boxShadow: 2,
            }}
          >
            <CardHeader
              title="NAS Security"
              sx={headerStyles}
              action={
                canEdit && (
                  <IconButton
                    aria-label="edit"
                    onClick={handleEditOperatorSecurityClick}
                  >
                    <EditIcon color={"primary"} />
                  </IconButton>
                )
              }
            />
            <CardContent>
              <Grid container spacing={1}>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body2" color="text.secondary">
                    Ciphering Preference
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
                    {operator?.security?.cipheringOrder?.length ? (
                      operator.security.cipheringOrder.map((alg, idx) => (
                        <Chip
                          key={idx}
                          label={`${idx + 1}. ${alg}`}
                          variant="outlined"
                          color={alg === "NEA0" ? "warning" : "primary"}
                        />
                      ))
                    ) : (
                      <Typography variant="body1">N/A</Typography>
                    )}
                  </Box>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body2" color="text.secondary">
                    Integrity Preference
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
                    {operator?.security?.integrityOrder?.length ? (
                      operator.security.integrityOrder.map((alg, idx) => (
                        <Chip
                          key={idx}
                          label={`${idx + 1}. ${alg}`}
                          variant="outlined"
                          color={alg === "NIA0" ? "warning" : "primary"}
                        />
                      ))
                    ) : (
                      <Typography variant="body1">N/A</Typography>
                    )}
                  </Box>
                </Grid>
              </Grid>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

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
