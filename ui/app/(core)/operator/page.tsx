"use client";

import React, { useState, useEffect } from "react";
import { Box, IconButton, Alert, Typography, Chip } from "@mui/material";
import { getOperator } from "@/queries/operator";
import { useCookies } from "react-cookie";
import { Edit as EditIcon } from "@mui/icons-material";
import EditOperatorIdModal from "@/components/EditOperatorIdModal";
import EditOperatorCodeModal from "@/components/EditOperatorCodeModal";
import EditSupportedTACsModal from "@/components/EditSupportedTacsModal";
import EditOperatorSstModal from "@/components/EditOperatorSstModal";
import EditOperatorSdModal from "@/components/EditOperatorSdModal";

import Grid from "@mui/material/Grid2";

interface OperatorData {
  mcc: string;
  mnc: string;
  supportedTacs: string[];
  sst: number;
  sd: number;
}

const Operator = () => {
  const [cookies] = useCookies(["user_token"]);

  const [operator, setOperator] = useState<OperatorData | null>(null);
  const [isEditOperatorIdModalOpen, setEditOperatorIdModalOpen] = useState(false);
  const [isEditOperatorCodeModalOpen, setEditOperatorCodeModalOpen] = useState(false);
  const [isEditSupportedTACsModalOpen, setEditSupportedTACsModalOpen] = useState(false);
  const [isEditOperatorSstModalOpen, setEditOperatorSstModalOpen] = useState(false);
  const [isEditOperatorSdModalOpen, setEditOperatorSdModalOpen] = useState(false);
  const [alert, setAlert] = useState<{ message: string; severity: "success" | "error" | null }>({
    message: "",
    severity: null,
  });

  const fetchOperator = async () => {
    try {
      const data = await getOperator(cookies.user_token);
      setOperator(data);
    } catch (error) {
      console.error("Error fetching operator ID:", error);
    }
  };

  useEffect(() => {
    fetchOperator();
  }, []);

  const handleEditOperatorIdClick = () => setEditOperatorIdModalOpen(true);
  const handleEditOperatorCodeClick = () => setEditOperatorCodeModalOpen(true);
  const handleEditSupportedTACsClick = () => setEditSupportedTACsModalOpen(true);
  const handleEditOperatorSstClick = () => setEditOperatorSstModalOpen(true);
  const handleEditOperatorSdClick = () => setEditOperatorSdModalOpen(true);

  const handleEditOperatorIdModalClose = () => setEditOperatorIdModalOpen(false);
  const handleEditOperatorCodeModalClose = () => setEditOperatorCodeModalOpen(false);
  const handleEditSupportedTACsModalClose = () => setEditSupportedTACsModalOpen(false);
  const handleEditOperatorSstModalClose = () => setEditOperatorSstModalOpen(false);
  const handleEditOperatorSdModalClose = () => setEditOperatorSdModalOpen(false);

  const handleEditOperatorIdSuccess = () => {
    fetchOperator();
    setAlert({ message: "Operator ID updated successfully!", severity: "success" });
  };

  const handleEditOperatorCodeSuccess = () => {
    setAlert({ message: "Operator Code updated successfully!", severity: "success" });
  };

  const handleEditSupportedTACsSuccess = () => {
    fetchOperator();
    setAlert({ message: "Supported TACs updated successfully!", severity: "success" });
  }

  const handleEditOperatorSstSuccess = () => {
    fetchOperator();
    setAlert({ message: "Operator SST updated successfully!", severity: "success" });
  };

  const handleEditOperatorSdSuccess = () => {
    fetchOperator();
    setAlert({ message: "Operator SD updated successfully!", severity: "success" });
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
        <Grid size={4}>
          <Typography variant="h6">Operator ID</Typography>
        </Grid>
        <Grid size={4}>
          <Typography variant="body1">{operator ? `${operator.mcc}${operator.mnc}` : "N/A"}</Typography>
        </Grid>
        <Grid size={4}>
          <IconButton aria-label="edit" onClick={handleEditOperatorIdClick}>
            <EditIcon />
          </IconButton>
        </Grid>

        <Grid size={4}>
          <Typography variant="h6">Operator Code</Typography>
        </Grid>
        <Grid size={4}>
          <Typography variant="body1">******</Typography>
        </Grid>
        <Grid size={4}>
          <IconButton aria-label="edit" onClick={handleEditOperatorCodeClick}>
            <EditIcon />
          </IconButton>
        </Grid>

        <Grid size={4}>
          <Typography variant="h6">Supported Tracking Area Codes (TAC's)</Typography>
        </Grid>
        <Grid size={4}>
          <Box sx={{ display: "flex", flexWrap: "wrap", gap: 1 }}>
            {operator?.supportedTacs?.length ? (
              operator.supportedTacs.map((tac, index) => (
                <Chip key={index} label={tac} variant="outlined" />
              ))
            ) : (
              <Typography variant="body1">No TACs available.</Typography>
            )}
          </Box>
        </Grid>
        <Grid size={4}>
          <IconButton aria-label="edit" onClick={handleEditSupportedTACsClick}>
            <EditIcon />
          </IconButton>
        </Grid>
        <Grid size={4}>
          <Typography variant="h6">Slice Service Type (SST)</Typography>
        </Grid>
        <Grid size={4}>
          <Typography variant="body1">{operator ? `${operator.sst}` : "N/A"}</Typography>
        </Grid>
        <Grid size={4}>
          <IconButton aria-label="edit" onClick={handleEditOperatorSstClick}>
            <EditIcon />
          </IconButton>
        </Grid>
        <Grid size={4}>
          <Typography variant="h6">Service Differentiator (SD)</Typography>
        </Grid>
        <Grid size={4}>
          <Typography variant="body1">{operator ? `${operator.sd}` : "N/A"}</Typography>
        </Grid>
        <Grid size={4}>
          <IconButton aria-label="edit" onClick={handleEditOperatorSdClick}>
            <EditIcon />
          </IconButton>
        </Grid>
      </Grid>

      <EditOperatorIdModal
        open={isEditOperatorIdModalOpen}
        onClose={handleEditOperatorIdModalClose}
        onSuccess={handleEditOperatorIdSuccess}
        initialData={
          operator || {
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
      <EditSupportedTACsModal
        open={isEditSupportedTACsModalOpen}
        onClose={handleEditSupportedTACsModalClose}
        onSuccess={handleEditSupportedTACsSuccess}
        initialData={
          operator || {
            supportedTacs: [""],
          }
        }
      />
      <EditOperatorSstModal
        open={isEditOperatorSstModalOpen}
        onClose={handleEditOperatorSstModalClose}
        onSuccess={handleEditOperatorSstSuccess}
        initialData={
          operator || {
            sst: 0,
          }
        }
      />
      <EditOperatorSdModal
        open={isEditOperatorSdModalOpen}
        onClose={handleEditOperatorSdModalClose}
        onSuccess={handleEditOperatorSdSuccess}
        initialData={
          operator || {
            sd: 0,
          }
        }
      />
    </Box>
  );
};

export default Operator;
