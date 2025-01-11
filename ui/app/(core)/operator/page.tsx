"use client";

import React, { useState, useEffect } from "react";
import { Box, IconButton, Alert, Typography, Chip } from "@mui/material";
import { getOperator } from "@/queries/operator";
import { useCookies } from "react-cookie";
import { Edit as EditIcon } from "@mui/icons-material";
import EditOperatorIdModal from "@/components/EditOperatorIdModal";
import EditOperatorCodeModal from "@/components/EditOperatorCodeModal";
import EditSupportedTACsModal from "@/components/EditSupportedTacsModal";
import Grid from "@mui/material/Grid2";

interface OperatorData {
  mcc: string;
  mnc: string;
  supportedTacs: string[];
}

const Operator = () => {
  const [cookies] = useCookies(["user_token"]);

  const [operator, setOperator] = useState<OperatorData | null>(null);
  const [isEditOperatorModalOpen, setEditOperatorModalOpen] = useState(false);
  const [isEditOperatorCodeModalOpen, setEditOperatorCodeModalOpen] = useState(false);
  const [isEditSupportedTACsModalOpen, setEditSupportedTACsModalOpen] = useState(false);
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

  const handleEditOperatorClick = () => setEditOperatorModalOpen(true);
  const handleEditOperatorCodeClick = () => setEditOperatorCodeModalOpen(true);
  const handleEditSupportedTACsClick = () => setEditSupportedTACsModalOpen(true);

  const handleEditOperatorModalClose = () => setEditOperatorModalOpen(false);
  const handleEditOperatorCodeModalClose = () => setEditOperatorCodeModalOpen(false);

  const handleEditSupportedTACsModalClose = () => setEditSupportedTACsModalOpen(false);

  const handleEditOperatorSuccess = () => {
    fetchOperator();
    setAlert({ message: "Operator updated successfully!", severity: "success" });
  };

  const handleEditOperatorCodeSuccess = () => {
    setAlert({ message: "Operator Code updated successfully!", severity: "success" });
  };

  const handleEditSupportedTACsSuccess = () => {
    fetchOperator();
    setAlert({ message: "Supported TACs updated successfully!", severity: "success" });
  }

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
          <IconButton aria-label="edit" onClick={handleEditOperatorClick}>
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

        {/* Supported TACs */}
        <Grid size={4}>
          <Typography variant="h6">Supported TACs</Typography>
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
      </Grid>

      <EditOperatorIdModal
        open={isEditOperatorModalOpen}
        onClose={handleEditOperatorModalClose}
        onSuccess={handleEditOperatorSuccess}
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
    </Box>
  );
};

export default Operator;
