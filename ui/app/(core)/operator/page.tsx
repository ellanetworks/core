"use client";

import React, { useCallback, useEffect, useState } from "react";
import {
  Box,
  IconButton,
  Alert,
  Typography,
  Chip,
  Card,
  CardHeader,
  CardContent,
  Tooltip,
} from "@mui/material";
import Grid from "@mui/material/Grid";
import { ContentCopy as CopyIcon, Edit as EditIcon } from "@mui/icons-material";
import { getOperator } from "@/queries/operator";
import EditOperatorIdModal from "@/components/EditOperatorIdModal";
import EditOperatorCodeModal from "@/components/EditOperatorCodeModal";
import EditOperatorTrackingModal from "@/components/EditOperatorTrackingModal";
import EditOperatorSliceModal from "@/components/EditOperatorSliceModal";
import EditOperatorHomeNetworkModal from "@/components/EditOperatorHomeNetworkModal";
import { useAuth } from "@/contexts/AuthContext";

interface OperatorData {
  id: { mcc: string; mnc: string };
  slice: { sst: number; sd: number };
  tracking: { supportedTacs: string[] };
  homeNetwork: { publicKey: string };
}

const MAX_WIDTH = 1400;

const Operator = () => {
  const { role, accessToken, authReady } = useAuth();

  const [operator, setOperator] = useState<OperatorData | null>(null);

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

  const [alert, setAlert] = useState<{
    message: string;
    severity: "success" | "error" | null;
  }>({ message: "", severity: null });

  const canEdit = role === "Admin" || role === "Network Manager";

  const fetchOperator = useCallback(async () => {
    if (!authReady || !accessToken) return;
    try {
      const data = await getOperator(accessToken);
      setOperator(data);
    } catch (error) {
      console.error("Error fetching operator information:", error);
      setAlert({ message: "Failed to load operator info.", severity: "error" });
    }
  }, [accessToken, authReady]);

  useEffect(() => {
    fetchOperator();
  }, [fetchOperator]);

  const handleEditOperatorIdClick = () => setEditOperatorIdModalOpen(true);
  const handleEditOperatorCodeClick = () => setEditOperatorCodeModalOpen(true);
  const handleEditOperatorTrackingClick = () =>
    setEditOperatorTrackingModalOpen(true);
  const handleEditOperatorSliceClick = () =>
    setEditOperatorSliceModalOpen(true);
  const handleEditOperatorHomeNetworkClick = () =>
    setEditOperatorHomeNetworkModalOpen(true);

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

  const handleEditOperatorIdSuccess = () => {
    fetchOperator();
    setAlert({
      message: "Operator ID updated successfully!",
      severity: "success",
    });
  };
  const handleEditOperatorCodeSuccess = () => {
    setAlert({
      message: "Operator Code updated successfully!",
      severity: "success",
    });
  };
  const handleEditOperatorTrackingSuccess = () => {
    fetchOperator();
    setAlert({
      message: "Operator Tracking information updated successfully!",
      severity: "success",
    });
  };
  const handleEditOperatorSliceSuccess = () => {
    fetchOperator();
    setAlert({
      message: "Operator Slice information updated successfully!",
      severity: "success",
    });
  };
  const handleEditOperatorHomeNetworkSuccess = () => {
    fetchOperator();
    setAlert({
      message: "Operator Home Network information updated successfully!",
      severity: "success",
    });
  };

  const handleCopyPublicKey = () => {
    if (operator?.homeNetwork.publicKey) {
      navigator.clipboard.writeText(operator.homeNetwork.publicKey);
      setAlert({
        message: "Public Key copied to clipboard!",
        severity: "success",
      });
    }
  };

  const headerStyles = {
    backgroundColor: "#F5F5F5",
    color: "#000000ff",
    borderTopLeftRadius: 12,
    borderTopRightRadius: 12,
    "& .MuiCardHeader-title": { color: "#000000ff" },
    "& .MuiIconButton-root": { color: "#000000ff" },
  };

  const descriptionText =
    "Review and configure your operator identifiers and core settings.";

  return (
    <Box sx={{ p: 4, maxWidth: MAX_WIDTH, mx: "auto" }}>
      <Typography variant="h4" sx={{ mb: 1 }}>
        Operator
      </Typography>

      <Typography variant="body1" color="text.secondary" sx={{ mb: 3 }}>
        {descriptionText}
      </Typography>

      {alert.severity && (
        <Box sx={{ mb: 3 }}>
          <Alert
            severity={alert.severity}
            onClose={() => setAlert({ message: "", severity: null })}
          >
            {alert.message}
          </Alert>
        </Box>
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
                      color={"primary"}
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
                  <Typography variant="body1">
                    {operator ? `${operator.slice.sd}` : "N/A"}
                  </Typography>
                </Grid>
              </Grid>
            </CardContent>
          </Card>
        </Grid>

        <Grid size={{ xs: 12, sm: 12, md: 12 }}>
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
                  <IconButton onClick={handleCopyPublicKey} sx={{ ml: 1 }}>
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
          initialData={operator?.slice || { sst: 0, sd: 0 }}
        />
      )}
      {isEditOperatorHomeNetworkModalOpen && (
        <EditOperatorHomeNetworkModal
          open
          onClose={handleEditOperatorHomeNetworkModalClose}
          onSuccess={handleEditOperatorHomeNetworkSuccess}
        />
      )}
    </Box>
  );
};

export default Operator;
