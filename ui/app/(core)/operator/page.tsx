"use client";

import React, { useState, useEffect } from "react";
import { Box, IconButton, Alert, Typography, Chip } from "@mui/material";
import { getOperator } from "@/queries/operator";
import { useCookies } from "react-cookie";
import { Edit as EditIcon } from "@mui/icons-material";
import EditOperatorIdModal from "@/components/EditOperatorIdModal";
import EditOperatorCodeModal from "@/components/EditOperatorCodeModal";
import EditOperatorTrackingModal from "@/components/EditOperatorTrackingModal";
import EditOperatorSliceModal from "@/components/EditOperatorSliceModal";
import Grid from "@mui/material/Grid2";

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
  }
}

const Operator = () => {
  const [cookies] = useCookies(["user_token"]);
  const [operator, setOperator] = useState<OperatorData | null>(null);
  const [isEditOperatorIdModalOpen, setEditOperatorIdModalOpen] = useState(false);
  const [isEditOperatorCodeModalOpen, setEditOperatorCodeModalOpen] = useState(false);
  const [isEditOperatorTrackingModalOpen, setEditOperatorTrackingModalOpen] = useState(false);
  const [isEditOperatorSliceModalOpen, setEditOperatorSliceModalOpen] = useState(false);
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

  const handleEditOperatorIdModalClose = () => setEditOperatorIdModalOpen(false);
  const handleEditOperatorCodeModalClose = () => setEditOperatorCodeModalOpen(false);
  const handleEditOperatorTrackingModalClose = () => setEditOperatorTrackingModalOpen(false);
  const handleEditOperatorSliceModalClose = () => setEditOperatorSliceModalOpen(false);

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
  }

  const handleEditOperatorSliceSuccess = () => {
    fetchOperator();
    setAlert({ message: "Operator Slice updated successfully!", severity: "success" });
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

      <Grid container spacing={3}>
        <Grid size={12}>
          <Typography variant="h5">
            Operator ID
            <IconButton aria-label="edit" onClick={handleEditOperatorIdClick}>
              <EditIcon />
            </IconButton>
          </Typography>
        </Grid>
        <Grid size={6}>
          <Typography variant="body1">Mobile Country Code (MCC)</Typography>
        </Grid>
        <Grid size={6}>
          <Typography variant="body1">{operator ? `${operator.id.mcc}` : "N/A"}</Typography>
        </Grid>
        <Grid size={6}>
          <Typography variant="body1">Mobile Network Code (MNC)</Typography>
        </Grid>
        <Grid size={6}>
          <Typography variant="body1">{operator ? `${operator.id.mnc}` : "N/A"}</Typography>
        </Grid>
      </Grid>

      <Box sx={{ marginBottom: 4 }} />

      <Grid container spacing={3}>
        <Grid size={12}>
          <Typography variant="h5">
            Operator Code
            <IconButton aria-label="edit" onClick={handleEditOperatorCodeClick}>
              <EditIcon />
            </IconButton>
          </Typography>
        </Grid>
      </Grid>

      <Box sx={{ marginBottom: 4 }} />

      <Grid container spacing={3}>
        <Grid size={12}>
          <Typography variant="h5">
            Tracking Information
            <IconButton aria-label="edit" onClick={handleEditOperatorTrackingClick}>
              <EditIcon />
            </IconButton>
          </Typography>
        </Grid>
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

      <Box sx={{ marginBottom: 4 }} />

      <Grid container spacing={3}>
        <Grid size={12}>
          <Typography variant="h5">
            Slice Information
            <IconButton aria-label="edit" onClick={handleEditOperatorSliceClick}>
              <EditIcon />
            </IconButton>
          </Typography>
        </Grid>
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
    </Box>
  );
};

export default Operator;
