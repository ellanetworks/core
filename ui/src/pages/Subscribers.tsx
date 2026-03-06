import React, { useMemo, useState } from "react";
import { Box, Typography, Button, CircularProgress, Chip } from "@mui/material";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  DataGrid,
  GridColDef,
  GridRenderCellParams,
  GridPaginationModel,
} from "@mui/x-data-grid";
import { useNavigate, Link } from "react-router-dom";
import {
  listSubscribers,
  type APISubscriberSummary,
  type ListSubscribersResponse,
} from "@/queries/subscribers";
import CreateSubscriberModal from "@/components/CreateSubscriberModal";
import EmptyState from "@/components/EmptyState";
import { useAuth } from "@/contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";

const MAX_WIDTH = 1400;

const SubscriberPage: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const theme = useTheme();
  const canEdit = role === "Admin" || role === "Network Manager";
  const navigate = useNavigate();

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: "#F5F5F5" } },
      }),
    [theme],
  );

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const { showSnackbar } = useSnackbar();

  const pageOneBased = paginationModel.page + 1;
  const perPage = paginationModel.pageSize;

  const { data, isLoading, refetch } = useQuery({
    queryKey: ["subscribers", pageOneBased, perPage],
    queryFn: (): Promise<ListSubscribersResponse> =>
      listSubscribers(accessToken || "", pageOneBased, perPage),
    enabled: authReady && !!accessToken,
    refetchInterval: 5000,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const rows: APISubscriberSummary[] = data?.items ?? [];
  const rowCount = data?.total_count ?? 0;

  const columns: GridColDef<APISubscriberSummary>[] = useMemo(() => {
    const base: GridColDef<APISubscriberSummary>[] = [
      {
        field: "imsi",
        headerName: "IMSI",
        flex: 1,
        minWidth: 200,
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Link
              to={`/subscribers/${params.row.imsi}`}
              style={{ textDecoration: "none" }}
              onClick={(e: React.MouseEvent) => e.stopPropagation()}
            >
              <Typography
                variant="body2"
                sx={{
                  fontFamily: "monospace",
                  color: "#4254FB",
                  textDecoration: "underline",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                {params.row.imsi}
              </Typography>
            </Link>
          </Box>
        ),
      },
      { field: "policyName", headerName: "Policy", flex: 0.8, minWidth: 140 },
      {
        field: "registration",
        headerName: "Registration",
        width: 140,
        minWidth: 120,
        valueGetter: (_v, row) => Boolean(row?.status?.registered),
        sortComparator: (v1, v2) => Number(v1) - Number(v2),
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => {
          const registered = Boolean(params.row?.status?.registered);
          return (
            <Chip
              size="small"
              label={registered ? "Registered" : "Deregistered"}
              color={registered ? "success" : "default"}
              variant="filled"
            />
          );
        },
      },
      {
        field: "ipAddress",
        headerName: "IP Address",
        width: 140,
        minWidth: 120,
        valueGetter: (_v, row: APISubscriberSummary) =>
          row?.status?.ipAddress ?? "",
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => {
          const ip = params.row?.status?.ipAddress ?? "";
          return (
            <Chip
              size="small"
              label={ip || "N/A"}
              color={ip ? "success" : "default"}
              variant="filled"
              sx={{ fontSize: "0.75rem" }}
            />
          );
        },
      },
    ];

    return base;
  }, []);

  const columnGroupingModel = [
    {
      groupId: "statusGroup",
      headerName: "Status",
      children: [
        { field: "registration" },
        { field: "session" },
        { field: "ipAddress" },
      ],
    },
  ];

  const descriptionText =
    "Manage subscribers connecting to your private network. After creating a subscriber here, you can emit a SIM card with the corresponding IMSI, Key and OPc.";

  if (!authReady) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
        <CircularProgress />
      </Box>
    );
  }

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
      {isLoading && !data ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : !isLoading && rowCount === 0 ? (
        <EmptyState
          primaryText="No subscriber found."
          secondaryText="Create a new subscriber."
          extraContent={
            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>
          }
          button={canEdit}
          buttonText="Create"
          onCreate={() => setCreateModalOpen(true)}
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
            <Typography variant="h4">Subscribers ({rowCount})</Typography>
            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>
            {canEdit && (
              <Button
                variant="contained"
                color="success"
                onClick={() => setCreateModalOpen(true)}
                sx={{ maxWidth: 200 }}
              >
                Create
              </Button>
            )}
          </Box>

          <Box
            sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}
          >
            <ThemeProvider theme={gridTheme}>
              <DataGrid<APISubscriberSummary>
                rows={rows}
                columns={columns}
                getRowId={(row) => row.imsi}
                columnGroupingModel={columnGroupingModel}
                disableRowSelectionOnClick
                paginationMode="server"
                rowCount={rowCount}
                paginationModel={paginationModel}
                onPaginationModelChange={setPaginationModel}
                pageSizeOptions={[10, 25, 50, 100]}
                sortingMode="server"
                disableColumnMenu
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
                  "& .MuiDataGrid-columnHeaderTitle": { fontWeight: "bold" },
                }}
              />
            </ThemeProvider>
          </Box>
        </>
      )}

      {isCreateModalOpen && (
        <CreateSubscriberModal
          open
          onClose={() => setCreateModalOpen(false)}
          onSuccess={() => {
            refetch();
            showSnackbar("Subscriber created successfully.", "success");
          }}
        />
      )}
    </Box>
  );
};

export default SubscriberPage;
