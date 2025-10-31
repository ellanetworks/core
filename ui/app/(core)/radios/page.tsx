"use client";

import React, { useMemo, useState } from "react";
import {
  Box,
  Typography,
  CircularProgress,
  Alert,
  Collapse,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import {
  DataGrid,
  type GridColDef,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import {
  listRadios,
  type APIRadio,
  type ListRadiosResponse,
} from "@/queries/radios";
import EmptyState from "@/components/EmptyState";
import { useAuth } from "@/contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";

const MAX_WIDTH = 1400;

const Radio = () => {
  const { accessToken } = useAuth();
  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const { data, isLoading } = useQuery<ListRadiosResponse>({
    queryKey: [
      "radios",
      accessToken,
      paginationModel.page,
      paginationModel.pageSize,
    ],
    queryFn: async () => {
      const pageOneBased = paginationModel.page + 1;
      return listRadios(
        accessToken || "",
        pageOneBased,
        paginationModel.pageSize,
      );
    },
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
  });

  const rows: APIRadio[] = data?.items ?? [];
  const rowCount: number = data?.total_count ?? 0;

  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: "#F5F5F5" } },
      }),
    [theme],
  );

  const columns: GridColDef<APIRadio>[] = useMemo(
    () => [
      { field: "id", headerName: "ID", flex: 0.6, minWidth: 160 },
      { field: "name", headerName: "Name", flex: 1, minWidth: 200 },
      { field: "address", headerName: "Address", flex: 1, minWidth: 240 },
    ],
    [],
  );

  const descriptionText =
    "View connected radios and their network locations. Radios will automatically appear here once connected.";

  return (
    <Box
      sx={{
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

      {isLoading && rowCount === 0 ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : rowCount === 0 ? (
        <EmptyState
          primaryText="No radio found."
          secondaryText="Connected radios will automatically appear here."
          extraContent={
            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>
          }
          button={false}
          buttonText="Create"
          onCreate={() => {}}
        />
      ) : (
        <>
          <Box
            sx={{
              width: "100%",
              maxWidth: MAX_WIDTH,
              px: { xs: 2, sm: 4 },
              mb: 3,
              display: "flex",
              flexDirection: "column",
              gap: 2,
            }}
          >
            <Typography variant="h4">Radios ({rowCount})</Typography>

            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>
          </Box>

          <Box
            sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}
          >
            <ThemeProvider theme={gridTheme}>
              <DataGrid<APIRadio>
                rows={rows}
                columns={columns}
                getRowId={(row) => row.id}
                paginationMode="server"
                rowCount={rowCount}
                paginationModel={paginationModel}
                onPaginationModelChange={setPaginationModel}
                pageSizeOptions={[10, 25, 50, 100]}
                sortingMode="server"
                disableColumnMenu
                disableRowSelectionOnClick
                columnVisibilityModel={{ id: !isSmDown }}
                sx={{
                  width: "100%",
                  border: 1,
                  borderColor: "divider",
                  "& .MuiDataGrid-cell": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-columnHeaders": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                    backgroundColor: "#F5F5F5",
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
