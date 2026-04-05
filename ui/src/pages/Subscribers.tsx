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
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const SubscriberPage: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const theme = useTheme();
  const canEdit = role === "Admin" || role === "Network Manager";
  const navigate = useNavigate();

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: theme.palette.backgroundSubtle } },
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
        minWidth: 150,
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
                  color: theme.palette.link,
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
      {
        field: "profile_name",
        headerName: "Profile",
        flex: 0.8,
        minWidth: 100,
      },
      {
        field: "radio",
        headerName: "Radio",
        flex: 0.8,
        minWidth: 100,
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => {
          const radioName = params.row.radio;
          if (!radioName) {
            return (
              <Box
                sx={{
                  display: "flex",
                  alignItems: "center",
                  width: "100%",
                  height: "100%",
                }}
              >
                <Typography variant="body2" color="text.secondary">
                  —
                </Typography>
              </Box>
            );
          }
          return (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                width: "100%",
                height: "100%",
              }}
            >
              <Link
                to={`/radios/${encodeURIComponent(radioName)}`}
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
                  {radioName}
                </Typography>
              </Link>
            </Box>
          );
        },
      },
      {
        field: "registration",
        headerName: "Registration",
        flex: 0.6,
        minWidth: 110,
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
        field: "pduSessions",
        headerName: "PDU Sessions",
        flex: 0.5,
        minWidth: 100,
        valueGetter: (_v, row: APISubscriberSummary) =>
          row?.status?.num_pdu_sessions ?? 0,
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => {
          const count = params.row?.status?.num_pdu_sessions ?? 0;
          return (
            <Chip
              size="small"
              label={count}
              color={count > 0 ? "success" : "default"}
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
      children: [{ field: "registration" }, { field: "pduSessions" }],
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
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
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
          readOnlyHint="Ask an administrator to create a subscriber."
        />
      ) : (
        <>
          <Box
            sx={{
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
              }}
            />
          </ThemeProvider>
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
