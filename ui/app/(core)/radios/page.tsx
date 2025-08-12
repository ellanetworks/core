"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import {
  Box,
  Typography,
  CircularProgress,
  Alert,
  Collapse,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import { DataGrid, GridColDef } from "@mui/x-data-grid";
import { listRadios } from "@/queries/radios";
import EmptyState from "@/components/EmptyState";
import { useCookies } from "react-cookie";
import { ThemeProvider, createTheme } from "@mui/material/styles";

interface RadioData {
  id: string;
  name: string;
  address: string;
}

const MAX_WIDTH = 1400;

const Radio = () => {
  const [cookies] = useCookies(["user_token"]);
  const [radios, setRadios] = useState<RadioData[]>([]);
  const [loading, setLoading] = useState(true);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));

  const outerTheme = useTheme();

  const gridTheme = React.useMemo(
    () =>
      createTheme(outerTheme, {
        palette: {
          DataGrid: {
            headerBg: "#F5F5F5",
          },
        },
      }),
    [outerTheme],
  );

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

  const columns: GridColDef[] = useMemo(
    () => [
      { field: "id", headerName: "ID", flex: 0.6, minWidth: 160 },
      { field: "name", headerName: "Name", flex: 1, minWidth: 200 },
      { field: "address", headerName: "Address", flex: 1, minWidth: 240 },
    ],
    [],
  );

  return (
    <Box
      sx={{
        minHeight: "100vh",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        pt: 6,
        pb: 4,
      }}
    >
      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}>
        <Collapse in={!!alert.message}>
          <Alert
            severity="success"
            onClose={() => setAlert({ message: "" })}
            sx={{ mb: 2 }}
          >
            {alert.message}
          </Alert>
        </Collapse>
      </Box>

      {loading ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : radios.length === 0 ? (
        <EmptyState
          primaryText="No radio found."
          secondaryText="Connected radios will automatically appear here."
          button={false}
          buttonText="Create"
          onCreate={() => {}}
        />
      ) : (
        <>
          {/* Header */}
          <Box
            sx={{
              width: "100%",
              maxWidth: MAX_WIDTH,
              px: { xs: 2, sm: 4 },
              mb: 3,
              display: "flex",
              flexDirection: { xs: "column", sm: "row" },
              justifyContent: "space-between",
              alignItems: { xs: "flex-start", sm: "center" },
              gap: 2,
            }}
          >
            <Typography variant="h4">Radios ({radios.length})</Typography>
          </Box>

          {/* Grid */}
          <Box sx={{ width: "100%", maxWidth: MAX_WIDTH }}>
            <ThemeProvider theme={gridTheme}>
              <DataGrid
                rows={radios}
                columns={columns}
                getRowId={(row) => row.id}
                disableRowSelectionOnClick
                columnVisibilityModel={{
                  id: !isSmDown,
                }}
                sx={{
                  width: "100%",
                  height: { xs: 460, sm: 560, md: 640 },
                  border: 1,
                  borderColor: "divider",
                  "& .MuiDataGrid-cell": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-columnHeaders": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-footerContainer": {
                    borderTop: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-columnHeaderTitle": { fontWeight: "bold" },
                }}
              />
            </ThemeProvider>
          </Box>
        </>
      )}
    </Box>
  );
};

export default Radio;
