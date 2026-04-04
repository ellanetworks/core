import React, { useMemo, useState } from "react";
import { Box, Typography, Chip, CircularProgress } from "@mui/material";
import { ThemeProvider, useTheme } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import {
  DataGrid,
  type GridColDef,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import { Link } from "react-router-dom";

import {
  listRadios,
  type APIRadio,
  type ListRadiosResponse,
} from "@/queries/radios";
import EmptyState from "@/components/EmptyState";
import { useAuth } from "@/contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";
import { MAX_WIDTH, PAGE_PADDING_X as PAGE_PAD } from "@/utils/layout";
import { useRadiosContext } from "./types";

export default function RadiosListTab() {
  const { gridTheme } = useRadiosContext();
  const { accessToken } = useAuth();
  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const { data, isLoading } = useQuery<ListRadiosResponse>({
    queryKey: ["radios", paginationModel.page, paginationModel.pageSize],
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
    placeholderData: (prev) => prev,
  });

  const rows: APIRadio[] = data?.items ?? [];
  const rowCount: number = data?.total_count ?? 0;

  const columns: GridColDef<APIRadio>[] = useMemo(
    () => [
      {
        field: "name",
        headerName: "Name",
        flex: 1,
        minWidth: 140,
        renderCell: (params) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Link
              to={`/radios/${encodeURIComponent(params.row.name)}`}
              style={{ textDecoration: "none" }}
              onClick={(e: React.MouseEvent) => e.stopPropagation()}
            >
              <Typography
                variant="body2"
                sx={{
                  color: theme.palette.link,
                  textDecoration: "underline",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                {params.row.name}
              </Typography>
            </Link>
          </Box>
        ),
      },
      { field: "id", headerName: "ID", flex: 0.6, minWidth: 100 },
      {
        field: "ran_node_type",
        headerName: "Type",
        flex: 0.4,
        minWidth: 80,
        renderCell: (params) => {
          const t = params.row.ran_node_type;
          const color =
            t === "gNB"
              ? "primary"
              : t === "ng-eNB"
                ? "secondary"
                : t === "N3IWF"
                  ? "warning"
                  : "default";
          return (
            <Chip size="small" label={t} color={color} variant="outlined" />
          );
        },
      },
      { field: "address", headerName: "Address", flex: 1, minWidth: 120 },
    ],
    [theme],
  );

  const descriptionText =
    "View connected radios and their network locations. Radios will automatically appear here once connected.";

  return (
    <>
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
        />
      ) : (
        <>
          <Box
            sx={{
              width: "100%",
              maxWidth: MAX_WIDTH,
              mx: "auto",
              px: PAGE_PAD,
              mb: 3,
              display: "flex",
              flexDirection: "column",
              gap: 2,
              mt: 2,
            }}
          >
            <Typography variant="h4">Radios ({rowCount})</Typography>

            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>
          </Box>

          <Box
            sx={{
              width: "100%",
              maxWidth: MAX_WIDTH,
              mx: "auto",
              px: PAGE_PAD,
            }}
          >
            <ThemeProvider theme={gridTheme}>
              <DataGrid<APIRadio>
                rows={rows}
                columns={columns}
                getRowId={(row) => row.address}
                paginationMode="server"
                rowCount={rowCount}
                paginationModel={paginationModel}
                onPaginationModelChange={setPaginationModel}
                pageSizeOptions={[10, 25, 50, 100]}
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
                  },
                  "& .MuiDataGrid-footerContainer": {
                    borderTop: "1px solid",
                    borderColor: "divider",
                  },
                }}
              />
            </ThemeProvider>
          </Box>
        </>
      )}
    </>
  );
}
