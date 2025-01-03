"use client";

import React, { useState, useEffect } from "react";
import { Box, IconButton, Alert, Typography } from "@mui/material";
import { getOperatorId } from "@/queries/operator";
import { useCookies } from "react-cookie";
import { Edit as EditIcon } from "@mui/icons-material";
import EditOperatorIdModal from "@/components/EditOperatorIdModal";
import EditOperatorCodeModal from "@/components/EditOperatorCodeModal";
import Grid from "@mui/material/Grid2";

interface OperatorIdData {
  mcc: string;
  mnc: string;
}

const Operator = () => {
  const [cookies] = useCookies(["user_token"]);

  const [operatorId, setOperatorId] = useState<OperatorIdData | null>(null);
  const [isEditOperatorIdModalOpen, setEditOperatorIdModalOpen] = useState(false);
  const [isEditOperatorCodeModalOpen, setEditOperatorCodeModalOpen] = useState(false);
  const [alert, setAlert] = useState<{ message: string; severity: "success" | "error" | null }>({
    message: "",
    severity: null,
  });

  const fetchOperatorId = async () => {
    try {
      const data = await getOperatorId(cookies.user_token);
      setOperatorId(data);
    } catch (error) {
      console.error("Error fetching operator ID:", error);
    }
  };

  useEffect(() => {
    fetchOperatorId();
  }, []);

  const handleEditOperatorIdClick = () => setEditOperatorIdModalOpen(true);
  const handleEditOperatorCodeClick = () => setEditOperatorCodeModalOpen(true);

  const handleEditOperatorIdModalClose = () => setEditOperatorIdModalOpen(false);
  const handleEditOperatorCodeModalClose = () => setEditOperatorCodeModalOpen(false);

  const handleEditOperatorIdSuccess = () => {
    fetchOperatorId();
    setAlert({ message: "Operator ID updated successfully!", severity: "success" });
  };

  const handleEditOperatorCodeSuccess = () => {
    setAlert({ message: "Operator Code updated successfully!", severity: "success" });
  };

  return (
    <Box sx={{ padding: 4, maxWidth: "1200px", margin: "0 auto" }}>
      <Typography variant="h4" component="h1" gutterBottom>
        Operator
      </Typography>
      <Grid container spacing={2}>
        <Grid size={8}>
          <Box sx={{ padding: 4, maxWidth: "1200px", margin: "0 auto" }}>

            {alert.severity && (
              <Alert severity={alert.severity} onClose={() => setAlert({ message: "", severity: null })}>
                {alert.message}
              </Alert>
            )}

            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", mb: 3 }}>
              <Typography variant="h6">Operator ID</Typography>
              <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
                <Typography variant="body1">{operatorId ? `${operatorId.mcc}${operatorId.mnc}` : "N/A"}</Typography>
                <IconButton aria-label="edit" onClick={handleEditOperatorIdClick}>
                  <EditIcon />
                </IconButton>
              </Box>
            </Box>

            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <Typography variant="h6">Operator Code</Typography>
              <Box sx={{ display: "flex", alignItems: "center", gap: 2 }}>
                <Typography variant="body1">******</Typography>
                <IconButton aria-label="edit" onClick={handleEditOperatorCodeClick}>
                  <EditIcon />
                </IconButton>
              </Box>
            </Box>

            <EditOperatorIdModal
              open={isEditOperatorIdModalOpen}
              onClose={handleEditOperatorIdModalClose}
              onSuccess={handleEditOperatorIdSuccess}
              initialData={
                operatorId || {
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
          </Box>
        </Grid>
      </Grid>
    </Box>
  );
};

export default Operator;
