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
} from "@mui/material";
import Grid from "@mui/material/Grid";
import {
  ContentCopy as CopyIcon,
  Edit as EditIcon,
  Warning as WarningIcon,
} from "@mui/icons-material";
import { getOperator, type OperatorData } from "@/queries/operator";
import EditOperatorIdModal from "@/components/EditOperatorIdModal";
import EditOperatorCodeModal from "@/components/EditOperatorCodeModal";
import EditOperatorTrackingModal from "@/components/EditOperatorTrackingModal";
import EditOperatorSliceModal from "@/components/EditOperatorSliceModal";
import EditOperatorHomeNetworkModal from "@/components/EditOperatorHomeNetworkModal";
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
  const [
    isEditOperatorHomeNetworkModalOpen,
    setEditOperatorHomeNetworkModalOpen,
  ] = useState(false);
  const [isEditOperatorSecurityModalOpen, setEditOperatorSecurityModalOpen] =
    useState(false);

  const anyModalOpen =
    isEditOperatorIdModalOpen ||
    isEditOperatorCodeModalOpen ||
    isEditOperatorTrackingModalOpen ||
    isEditOperatorSliceModalOpen ||
    isEditOperatorHomeNetworkModalOpen ||
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
  const handleEditOperatorHomeNetworkClick = () =>
    setEditOperatorHomeNetworkModalOpen(true);
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
  const handleEditOperatorHomeNetworkModalClose = () =>
    setEditOperatorHomeNetworkModalOpen(false);
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
  const handleEditOperatorHomeNetworkSuccess = () => {
    queryClient.invalidateQueries({ queryKey: ["operator"] });
    showSnackbar(
      "Operator Home Network information updated successfully.",
      "success",
    );
  };
  const handleEditOperatorSecuritySuccess = () => {
    queryClient.invalidateQueries({ queryKey: ["operator"] });
    showSnackbar("NAS security algorithms updated successfully.", "success");
  };

  const handleCopyPublicKey = async () => {
    if (!operator?.homeNetwork.publicKey) return;
    if (!navigator.clipboard) {
      showSnackbar(
        "Clipboard API not available. Please use HTTPS or try a different browser.",
        "error",
      );
      return;
    }
    try {
      await navigator.clipboard.writeText(operator.homeNetwork.publicKey);
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
    "& .MuiCardHeader-title": { color: "text.primary" },
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
                    MCC
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body1">
                    {operator?.id.mcc || "N/A"}
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body2" color="text.secondary">
                    MNC
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
                    OP
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
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                Supported TACs
              </Typography>
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
                  <Typography variant="body1">No TACs available.</Typography>
                )}
              </Box>
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
                    SST
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body1">
                    {operator ? `${operator.slice.sst}` : "N/A"}
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body2" color="text.secondary">
                    SD
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
              title="Home Network Information"
              sx={headerStyles}
              action={
                canEdit && (
                  <IconButton
                    aria-label="edit"
                    onClick={handleEditOperatorHomeNetworkClick}
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
                    Encryption
                  </Typography>
                </Grid>
                <Grid size={{ xs: 6 }}>
                  <Typography variant="body1">ECIES - Profile A</Typography>
                </Grid>

                <Grid size={{ xs: 6 }}>
                  <Typography variant="body2" color="text.secondary">
                    Public Key
                  </Typography>
                </Grid>
                <Grid
                  size={{ xs: 6 }}
                  sx={{ display: "flex", alignItems: "center", minWidth: 0 }}
                >
                  <Tooltip
                    title={operator?.homeNetwork.publicKey || "N/A"}
                    arrow
                  >
                    <Typography
                      variant="body1"
                      sx={{
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                        maxWidth: 280,
                      }}
                    >
                      {operator?.homeNetwork.publicKey || "N/A"}
                    </Typography>
                  </Tooltip>
                  <IconButton
                    onClick={handleCopyPublicKey}
                    sx={{ ml: 1, my: -0.5 }}
                  >
                    <CopyIcon fontSize="small" color={"primary"} />
                  </IconButton>
                </Grid>

                <Grid size={{ xs: 6 }}>
                  <Typography variant="body2" color="text.secondary">
                    Private Key
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
                          color="primary"
                          icon={
                            alg === "NEA0" ? (
                              <WarningIcon fontSize="small" />
                            ) : undefined
                          }
                          sx={
                            alg === "NEA0"
                              ? { "& .MuiChip-icon": { color: "warning.main" } }
                              : undefined
                          }
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
                          color="primary"
                          icon={
                            alg === "NIA0" ? (
                              <WarningIcon fontSize="small" />
                            ) : undefined
                          }
                          sx={
                            alg === "NIA0"
                              ? { "& .MuiChip-icon": { color: "warning.main" } }
                              : undefined
                          }
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
      {isEditOperatorHomeNetworkModalOpen && (
        <EditOperatorHomeNetworkModal
          open
          onClose={handleEditOperatorHomeNetworkModalClose}
          onSuccess={handleEditOperatorHomeNetworkSuccess}
        />
      )}
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
