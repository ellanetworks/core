import React, { useState, useMemo } from "react";
import {
  Box,
  Typography,
  Button,
  CircularProgress,
  Chip,
  Stack,
} from "@mui/material";
import { ThemeProvider, useTheme } from "@mui/material/styles";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import {
  DataGrid,
  type GridColDef,
  type GridRenderCellParams,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import {
  listDataNetworks,
  type ListDataNetworksResponse,
  type APIDataNetwork,
} from "@/queries/data_networks";
import CreateDataNetworkModal from "@/components/CreateDataNetworkModal";
import EmptyState from "@/components/EmptyState";
import { useNetworkingContext } from "./types";

export default function DataNetworksTab() {
  const { accessToken, canEdit, showSnackbar, gridTheme } =
    useNetworkingContext();
  const outerTheme = useTheme();

  const [pagination, setPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const {
    data: dnPage,
    isLoading: loading,
    refetch,
  } = useQuery<ListDataNetworksResponse>({
    queryKey: ["data-networks", pagination.page, pagination.pageSize],
    queryFn: () =>
      listDataNetworks(
        accessToken || "",
        pagination.page + 1,
        pagination.pageSize,
      ),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const rows: APIDataNetwork[] = dnPage?.items ?? [];
  const rowCount = dnPage?.total_count ?? 0;

  const [isCreateOpen, setCreateOpen] = useState(false);

  const description =
    "Manage the IP networks used by your subscribers. Data Network Names (DNNs) are used to identify different networks, and must be configured on the subscriber device to connect to the correct network.";

  const columns: GridColDef<APIDataNetwork>[] = useMemo(() => {
    return [
      {
        field: "name",
        headerName: "Name (DNN)",
        flex: 1,
        minWidth: 200,
        renderCell: (params: GridRenderCellParams<APIDataNetwork>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Link
              to={`/networking/data-networks/${params.row.name}`}
              style={{ textDecoration: "none" }}
              onClick={(e: React.MouseEvent) => e.stopPropagation()}
            >
              <Typography
                variant="body2"
                sx={{
                  color: outerTheme.palette.link,
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
      { field: "ip_pool", headerName: "IP Pool", flex: 1, minWidth: 180 },
      {
        field: "sessions",
        headerName: "Sessions",
        width: 120,
        valueGetter: (_v, row) => row.status?.sessions ?? 0,
        renderCell: (params: GridRenderCellParams<APIDataNetwork>) => {
          const count = params.row.status?.sessions ?? 0;
          return (
            <Chip
              size="small"
              label={count}
              color={count > 0 ? "success" : "default"}
              variant="filled"
            />
          );
        },
      },
    ];
  }, [outerTheme]);

  return (
    <Box
      sx={{
        width: "100%",
        mt: 2,
      }}
    >
      {loading && rowCount === 0 ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : rowCount === 0 ? (
        <EmptyState
          primaryText="No data network found."
          secondaryText="Create a data network to assign subscribers and control egress."
          extraContent={
            <Typography variant="body1" color="textSecondary">
              {description}
            </Typography>
          }
          button={canEdit}
          buttonText="Create"
          onCreate={() => setCreateOpen(true)}
          readOnlyHint="Ask an administrator to create a data network."
        />
      ) : (
        <>
          <Box sx={{ mb: 3 }}>
            <Stack
              direction={{ xs: "column", sm: "row" }}
              spacing={2}
              sx={{
                alignItems: { xs: "stretch", sm: "center" },
                justifyContent: "space-between",
              }}
            >
              <Box>
                <Typography variant="h5" sx={{ mb: 0.5 }}>
                  Data Networks ({rowCount})
                </Typography>
                <Typography variant="body2" color="textSecondary">
                  {description}
                </Typography>
                {canEdit && (
                  <Button
                    variant="contained"
                    color="success"
                    onClick={() => setCreateOpen(true)}
                    sx={{ maxWidth: 200, mt: 2 }}
                  >
                    Create
                  </Button>
                )}
              </Box>
            </Stack>
          </Box>
          <ThemeProvider theme={gridTheme}>
            <DataGrid<APIDataNetwork>
              rows={rows}
              columns={columns}
              getRowId={(row) => row.name}
              paginationMode="server"
              rowCount={rowCount}
              paginationModel={pagination}
              onPaginationModelChange={setPagination}
              pageSizeOptions={[10, 25, 50, 100]}
              disableColumnMenu
              disableRowSelectionOnClick
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
        </>
      )}

      {isCreateOpen && (
        <CreateDataNetworkModal
          open
          onClose={() => setCreateOpen(false)}
          onSuccess={() => {
            refetch();
            showSnackbar("Data network created successfully.", "success");
          }}
        />
      )}
    </Box>
  );
}
