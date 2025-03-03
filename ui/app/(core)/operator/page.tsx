"use client";

import React, { useState, useEffect } from "react";
import {
  Box,
  IconButton,
  Alert,
  Typography,
  Chip,
  Card,
  CardHeader,
  Button,
  CardContent,
  CardActions,
  Tooltip
} from "@mui/material";
import { ContentCopy as CopyIcon, Edit as EditIcon } from "@mui/icons-material";
import { getOperator } from "@/queries/operator";
import { useCookies } from "react-cookie";
import EditOperatorIdModal from "@/components/EditOperatorIdModal";
import EditOperatorCodeModal from "@/components/EditOperatorCodeModal";
import EditOperatorTrackingModal from "@/components/EditOperatorTrackingModal";
import EditOperatorSliceModal from "@/components/EditOperatorSliceModal";
import EditOperatorHomeNetworkModal from "@/components/EditOperatorHomeNetworkModal";
import Grid from "@mui/material/Grid2";
import { useAuth } from "@/contexts/AuthContext";

interface OperatorData {
  id: {
    mcc: string;
    mnc: string;
  };
  slice: {
    sst: number;
    sd: number;
  };
  tracking: {
    supportedTacs: string[];
  };
  homeNetwork: {
    publicKey: string;
  };
}

const Operator = () => {
  const { role } = useAuth();
  const [cookies] = useCookies(["user_token"]);
  const [operator, setOperator] = useState<OperatorData | null>(null);
  const [isEditOperatorIdModalOpen, setEditOperatorIdModalOpen] = useState(false);
  const [isEditOperatorCodeModalOpen, setEditOperatorCodeModalOpen] = useState(false);
  const [isEditOperatorTrackingModalOpen, setEditOperatorTrackingModalOpen] = useState(false);
  const [isEditOperatorSliceModalOpen, setEditOperatorSliceModalOpen] = useState(false);
  const [isEditOperatorHomeNetworkModalOpen, setEditOperatorHomeNetworkModalOpen] = useState(false);
  const [alert, setAlert] = useState<{ message: string; severity: "success" | "error" | null }>({
    message: "",
    severity: null,
  });

  const fetchOperator = async () => {
    try {
      const data = await getOperator(cookies.user_token);
      setOperator(data);
    } catch (error) {
      console.error("Error fetching operator information:", error);
    }
  };

  useEffect(() => {
    fetchOperator();
  }, []);

  const handleEditOperatorIdClick = () => setEditOperatorIdModalOpen(true);
  const handleEditOperatorCodeClick = () => setEditOperatorCodeModalOpen(true);
  const handleEditOperatorTrackingClick = () => setEditOperatorTrackingModalOpen(true);
  const handleEditOperatorSliceClick = () => setEditOperatorSliceModalOpen(true);
  const handleEditOperatorHomeNetworkClick = () => setEditOperatorHomeNetworkModalOpen(true);

  const handleEditOperatorIdModalClose = () => setEditOperatorIdModalOpen(false);
  const handleEditOperatorCodeModalClose = () => setEditOperatorCodeModalOpen(false);
  const handleEditOperatorTrackingModalClose = () => setEditOperatorTrackingModalOpen(false);
  const handleEditOperatorSliceModalClose = () => setEditOperatorSliceModalOpen(false);
  const handleEditOperatorHomeNetworkModalClose = () => setEditOperatorHomeNetworkModalOpen(false);

  const handleEditOperatorIdSuccess = () => {
    fetchOperator();
    setAlert({ message: "Operator ID updated successfully!", severity: "success" });
  };

  const handleEditOperatorCodeSuccess = () => {
    setAlert({ message: "Operator Code updated successfully!", severity: "success" });
  };

  const handleEditOperatorTrackingSuccess = () => {
    fetchOperator();
    setAlert({ message: "Operator Tracking information updated successfully!", severity: "success" });
  };

  const handleEditOperatorSliceSuccess = () => {
    fetchOperator();
    setAlert({ message: "Operator Slice information updated successfully!", severity: "success" });
  };

  const handleEditOperatorHomeNetworkSuccess = () => {
    fetchOperator();
    setAlert({ message: "Operator Home Network information updated successfully!", severity: "success" });
  };

  const handleCopyPublicKey = () => {
    if (operator?.homeNetwork.publicKey) {
      navigator.clipboard.writeText(operator.homeNetwork.publicKey);
      setAlert({ message: "Public Key copied to clipboard!", severity: "success" });
    }
  };

  return (
    <Box sx={{ padding: 4, maxWidth: "1200px", margin: "0 auto" }}>
      <Typography variant="h4" component="h1" sx={{ marginBottom: 4 }}>
        Operator
      </Typography>

      {alert.severity && (
        <Alert severity={alert.severity} onClose={() => setAlert({ message: "", severity: null })}>
          {alert.message}
        </Alert>
      )}

      {/* Operator ID Card */}
      <Card variant="outlined" sx={{ marginBottom: 3, borderRadius: 2, boxShadow: 1, borderColor: "rgba(0, 0, 0, 0.12)" }}>
        <CardHeader title="Operator ID" />
        <CardContent>
          <Grid container spacing={2}>
            <Grid size={6}>
              <Typography variant="body1">Mobile Country Code (MCC)</Typography>
            </Grid>
            <Grid size={6}>
              <Typography variant="body1">{operator?.id.mcc || "N/A"}</Typography>
            </Grid>
            <Grid size={6}>
              <Typography variant="body1">Mobile Network Code (MNC)</Typography>
            </Grid>
            <Grid size={6}>
              <Typography variant="body1">{operator?.id.mnc || "N/A"}</Typography>
            </Grid>
          </Grid>
        </CardContent>
        {(role === "Admin" || role === "Network Manager") && (
          <CardActions>
            <IconButton aria-label="edit" onClick={handleEditOperatorIdClick}>
              <EditIcon />
            </IconButton>
          </CardActions>
        )}
      </Card>

      {/* Operator Code Card */}
      <Card variant="outlined" sx={{ marginBottom: 3, borderRadius: 2, boxShadow: 1, borderColor: "rgba(0, 0, 0, 0.12)" }}>
        <CardHeader title="Operator Code" />
        <CardContent>
          <Grid container spacing={2}>
            <Grid size={6}>
              <Typography variant="body1">Operator Code (OP)</Typography>
            </Grid>
            <Grid size={6}>
              <Typography variant="body1">{"***************"}</Typography>
            </Grid>
          </Grid>
        </CardContent>
        {(role === "Admin" || role === "Network Manager") && (
          <CardActions>
            <IconButton aria-label="edit" onClick={handleEditOperatorCodeClick}>
              <EditIcon />
            </IconButton>
          </CardActions>
        )}
      </Card>

      {/* Tracking Information Card */}
      <Card variant="outlined" sx={{ marginBottom: 3, borderRadius: 2, boxShadow: 1, borderColor: "rgba(0, 0, 0, 0.12)" }}>
        <CardHeader title="Tracking Information" />
        <CardContent>
          <Grid container spacing={2}>
            <Grid size={6}>
              <Typography variant="body1">Supported Tracking Area Codes (TAC's)</Typography>
            </Grid>
            <Grid size={6}>
              <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
                {operator?.tracking.supportedTacs?.length ? (
                  operator.tracking.supportedTacs.map((tac, index) => (
                    <Chip key={index} label={tac} variant="outlined" />
                  ))
                ) : (
                  <Typography variant="body1">No TACs available.</Typography>
                )}
              </Box>
            </Grid>
          </Grid>
        </CardContent>
        {(role === "Admin" || role === "Network Manager") && (
          <CardActions>
            <IconButton aria-label="edit" onClick={handleEditOperatorTrackingClick}>
              <EditIcon />
            </IconButton>
          </CardActions>
        )}
      </Card>

      <Card variant="outlined" sx={{ marginBottom: 3, borderRadius: 2, boxShadow: 1, borderColor: "rgba(0, 0, 0, 0.12)" }}>
        <CardHeader title="Slice Information" />
        <CardContent>
          <Grid container spacing={2}>
            <Grid size={6}>
              <Typography variant="body1">Slice Service Type (SST)</Typography>
            </Grid>
            <Grid size={6}>
              <Typography variant="body1">{operator ? `${operator.slice.sst}` : "N/A"}</Typography>
            </Grid>
            <Grid size={6}>
              <Typography variant="body1">Service Differentiator (SD)</Typography>
            </Grid>
            <Grid size={6}>
              <Typography variant="body1">{operator ? `${operator.slice.sd}` : "N/A"}</Typography>
            </Grid>
          </Grid>
        </CardContent>
        {(role === "Admin" || role === "Network Manager") && (
          <CardActions>
            <IconButton aria-label="edit" onClick={handleEditOperatorSliceClick}>
              <EditIcon />
            </IconButton>
          </CardActions>
        )}
      </Card>

      {/* Home Network Information Card */}
      <Card variant="outlined" sx={{ marginBottom: 3, borderRadius: 2, boxShadow: 1, borderColor: "rgba(0, 0, 0, 0.12)" }}>
        <CardHeader title="Home Network Information" />
        <CardContent>
          <Grid container spacing={2}>
            <Grid size={6}>
              <Typography variant="body1">Encryption</Typography>
            </Grid>
            <Grid size={6}>
              <Typography variant="body1">{"ECIES - Profile A"}</Typography>
            </Grid>
            <Grid size={6}>
              <Typography variant="body1">Public Key</Typography>
            </Grid>
            <Grid size={6} sx={{ display: "flex", alignItems: "center" }}>
              <Tooltip title={operator?.homeNetwork.publicKey || "N/A"} arrow>
                <Typography
                  variant="body1"
                  sx={{
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                    maxWidth: "250px",
                  }}
                >
                  {operator?.homeNetwork.publicKey || "N/A"}
                </Typography>
              </Tooltip>
              <IconButton onClick={handleCopyPublicKey} sx={{ marginLeft: 1 }}>
                <CopyIcon fontSize="small" />
              </IconButton>
            </Grid>
            <Grid size={6}>
              <Typography variant="body1">Private Key</Typography>
            </Grid>
            <Grid size={6}>
              <Typography variant="body1">{"***************"}</Typography>
            </Grid>
          </Grid>
        </CardContent>
        {(role === "Admin" || role === "Network Manager") && (
          <CardActions>
            <IconButton aria-label="edit" onClick={handleEditOperatorHomeNetworkClick}>
              <EditIcon />
            </IconButton>
          </CardActions>
        )}
      </Card>

      <EditOperatorIdModal
        open={isEditOperatorIdModalOpen}
        onClose={handleEditOperatorIdModalClose}
        onSuccess={handleEditOperatorIdSuccess}
        initialData={
          operator?.id || {
            mcc: "",
            mnc: "",
          }
        }
      />
      <EditOperatorCodeModal
        open={isEditOperatorCodeModalOpen}
        onClose={handleEditOperatorCodeModalClose}
        onSuccess={handleEditOperatorCodeSuccess}
      />
      <EditOperatorTrackingModal
        open={isEditOperatorTrackingModalOpen}
        onClose={handleEditOperatorTrackingModalClose}
        onSuccess={handleEditOperatorTrackingSuccess}
        initialData={
          operator?.tracking || {
            supportedTacs: [""],
          }
        }
      />
      <EditOperatorSliceModal
        open={isEditOperatorSliceModalOpen}
        onClose={handleEditOperatorSliceModalClose}
        onSuccess={handleEditOperatorSliceSuccess}
        initialData={
          operator?.slice || {
            sst: 0,
            sd: 0,
          }
        }
      />
      <EditOperatorHomeNetworkModal
        open={isEditOperatorHomeNetworkModalOpen}
        onClose={handleEditOperatorHomeNetworkModalClose}
        onSuccess={handleEditOperatorHomeNetworkSuccess}
      />
    </Box>
  );
};

export default Operator;
