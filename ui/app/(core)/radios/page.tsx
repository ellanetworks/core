"use client";

import React, { useCallback, useState, useEffect } from "react";
import {
  Box,
  Typography,
  CircularProgress,
  Alert,
  Collapse,
} from "@mui/material";
import { DataGrid, GridColDef } from "@mui/x-data-grid";
import { listRadios } from "@/queries/radios";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";

interface RadioData {
  id: string;
  name: string;
  address: string;
}

const Radio = () => {
  const [cookies] = useCookies(["user_token"]);
  const [radios, setRadios] = useState<RadioData[]>([]);
  const [loading, setLoading] = useState(true);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const fetchRadios = useCallback(async () => {
    setLoading(true);
    try {
      const data = await listRadios(cookies.user_token);
      setRadios(data);
    } catch (error) {
      console.error("Error fetching radios:", error);
    } finally {
      setLoading(false);
    }
  }, [cookies.user_token]);

  useEffect(() => {
    fetchRadios();
  }, [fetchRadios]);

  const columns: GridColDef[] = [
    { field: "id", headerName: "ID", flex: 1 },
    { field: "name", headerName: "Name", flex: 1 },
    { field: "address", headerName: "Address", flex: 1 },
  ];

  return (
    <Box
      sx={{
        height: "100vh",
        display: "flex",
        flexDirection: "column",
        justifyContent: "flex-start",
        alignItems: "center",
        paddingTop: 6,
        textAlign: "center",
      }}
    >
      <Box sx={{ width: "60%" }}>
        <Collapse in={!!alert.message}>
          <Alert
            severity="success"
            onClose={() => setAlert({ message: "" })}
            sx={{ marginBottom: 2 }}
          >
            {alert.message}
          </Alert>
        </Collapse>
      </Box>
      {loading ? (
        <Box
          sx={{
            display: "flex",
            justifyContent: "center",
            alignItems: "center",
          }}
        >
          <CircularProgress />
        </Box>
      ) : radios.length === 0 ? (
        <EmptyState
          primaryText="No radio found."
          secondaryText="Connected radios will automatically appear here."
          button={false}
          buttonText="Create"
          onCreate={() => console.log("Create radio")}
        />
      ) : (
        <>
          <Box
            sx={{
              marginBottom: 4,
              width: "60%",
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <Typography variant="h4" component="h1" gutterBottom>
              Radios ({radios.length})
            </Typography>
          </Box>
          <Box
            sx={{
              height: "80vh",
              width: "60%",
              "& .MuiDataGrid-root": {
                border: "none",
              },
              "& .MuiDataGrid-cell": {
                borderBottom: "none",
              },
              "& .MuiDataGrid-columnHeaders": {
                borderBottom: "none",
              },
              "& .MuiDataGrid-footerContainer": {
                borderTop: "none",
              },
            }}
          >
            <DataGrid
              rows={radios}
              columns={columns}
              getRowId={(row) => row.id}
              disableRowSelectionOnClick
            />
          </Box>
        </>
      )}
    </Box>
  );
};

export default Radio;
