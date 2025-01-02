"use client";

import React, { useState, useEffect } from "react";
import { Box, IconButton, Alert, Typography } from "@mui/material";
import { getOperatorId } from "@/queries/operator";
import { useRouter } from "next/navigation";
import { useCookies } from "react-cookie";
import Grid from "@mui/material/Grid2";
import {
  Edit as EditIcon,
} from "@mui/icons-material";
import EditOperatorIdModal from "@/components/EditOperatorIdModal";
import EditOperatorCodeModal from "@/components/EditOperatorCodeModal";


interface OperatorIdData {
  mcc: string;
  mnc: string;
}

const Operator = () => {
  const router = useRouter();
  const [cookies, setCookie, removeCookie] = useCookies(["user_token"]);

  if (!cookies.user_token) {
    router.push("/login");
  }

  const [operatorId, setOperatorId] = useState<{ mcc: string; mnc: string } | null>(null);

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

  const [mcc, setMcc] = useState("");
  const [mnc, setMnc] = useState("");
  const [mccError, setMccError] = useState("");
  const [mncError, setMncError] = useState("");
  const [isEdited, setIsEdited] = useState(false);
  const [isSaveDisabled, setSaveDisabled] = useState(true);
  const [editOperatorIdData, setEditOperatorIdData] = useState<OperatorIdData | null>(null);
  const [isEditOperatorIdModalOpen, setEditOperatorIdModalOpen] = useState(false);
  const [isEditOperatorCodeModalOpen, setEditOperatorCodeModalOpen] = useState(false);

  const [alert, setAlert] = useState<{ message: string; severity: "success" | "error" | null }>({
    message: "",
    severity: null,
  });

  const handleEditOperatorIdClick = (operatorId: any) => {
    const mappedOperatorId = {
      mcc: operatorId.mcc,
      mnc: operatorId.mnc,
    };

    setEditOperatorIdData(mappedOperatorId);
    setEditOperatorIdModalOpen(true);
  };

  const handleEditOperatorCodeClick = () => {
    setEditOperatorCodeModalOpen(true);
  }

  const handleEditOperatorIdModalClose = () => {
    setEditOperatorIdModalOpen(false);
    setEditOperatorIdData(null);
  };

  const handleEditOperatorCodeModalClose = () => {
    setEditOperatorCodeModalOpen(false);
  };

  const handleEditOperatorIdSuccess = () => {
    fetchOperatorId();
    setAlert({ message: "Operator ID updated successfully!", severity: "success" });
  };

  const handleEditOperatorCodeSuccess = () => {
    setAlert({ message: "Operator Code updated successfully!", severity: "success" });
  };

  return (
    <Box sx={{ padding: 4, maxWidth: "1200px", margin: "0 auto" }}>
      <Typography variant="h4" component="h1" gutterBottom sx={{ textAlign: "left", marginBottom: 4 }}>
        Operator
      </Typography>

      {alert.severity && (
        <Alert severity={alert.severity} onClose={() => setAlert({ message: "", severity: null })}>
          {alert.message}
        </Alert>
      )}

      <Grid container spacing={4} justifyContent="flex-start">
        <Grid size={4}>
          <Box
            sx={{
              border: "1px solid #ccc",
              borderRadius: 4,
              padding: 4,
              width: "100%",
              margin: "0 auto",
              textAlign: "center",
            }}
          >
            <Typography variant="h6" component="h2" gutterBottom>
              Operator ID
            </Typography>
            <Typography variant="body1" component="h2" gutterBottom>
              {operatorId ? operatorId.mcc + operatorId.mnc : "N/A"}
            </Typography>
            <IconButton
              aria-label="edit"
              onClick={() => handleEditOperatorIdClick(operatorId)}
            >
              <EditIcon />
            </IconButton>
            <EditOperatorIdModal
              open={isEditOperatorIdModalOpen}
              onClose={handleEditOperatorIdModalClose}
              onSuccess={handleEditOperatorIdSuccess}
              initialData={
                editOperatorIdData || {
                  mcc: "",
                  mnc: "",
                }
              }
            />
          </Box>
        </Grid>
        <Grid size={4}>
          <Box
            sx={{
              border: "1px solid #ccc",
              borderRadius: 4,
              padding: 4,
              width: "100%",
              margin: "0 auto",
              textAlign: "center",
            }}
          >
            <Typography variant="h6" component="h2" gutterBottom>
              Operator Code
            </Typography>
            <Typography variant="body1" component="h2" gutterBottom>
              ******
            </Typography>
            <IconButton
              aria-label="edit"
              onClick={() => handleEditOperatorCodeClick()}
            >
              <EditIcon />
            </IconButton>
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
