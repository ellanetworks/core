// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useState } from "react";
import { Box, Typography, Button, Chip } from "@mui/material";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useTheme } from "@mui/material/styles";
import {
  DataGrid,
  GridColDef,
  GridRenderCellParams,
  GridPaginationModel,
} from "@mui/x-data-grid";
import { Link } from "react-router-dom";
import {
  listSubscribers,
  type APISubscriberSummary,
  type ListSubscribersResponse,
} from "@/queries/subscribers";
import CreateSubscriberModal from "@/components/CreateSubscriberModal";
import EmptyState from "@/components/EmptyState";
import QueryState from "@/components/QueryState";
import AccessChip from "@/components/AccessChip";
import { useAuth } from "@/contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const SubscriberPage: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const theme = useTheme();
  const canEdit = role === "Admin" || role === "Network Manager";

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const { showSnackbar } = useSnackbar();

  const pageOneBased = paginationModel.page + 1;
  const perPage = paginationModel.pageSize;

  const subscribersQuery = useQuery({
    queryKey: ["subscribers", pageOneBased, perPage],
    queryFn: (): Promise<ListSubscribersResponse> =>
      listSubscribers(accessToken || "", pageOneBased, perPage),
    enabled: authReady && !!accessToken,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    // The poll is the retry; backoff would only delay the error reaching the UI.
    retry: false,
    placeholderData: (prev) => prev,
  });

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
                <Typography variant="body2" color="textSecondary">
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
        field: "access",
        headerName: "Access",
        flex: 0.4,
        minWidth: 90,
        valueGetter: (_v, row: APISubscriberSummary) =>
          row?.status?.radio_access_type ?? "",
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => {
          const rat = params.row?.status?.radio_access_type;
          if (!rat) return "—";
          return (
            <Box sx={{ display: "flex", alignItems: "center", height: "100%" }}>
              <AccessChip label={rat} />
            </Box>
          );
        },
      },
      {
        field: "sessions",
        headerName: "Sessions",
        flex: 0.5,
        minWidth: 100,
        valueGetter: (_v, row: APISubscriberSummary) =>
          row?.status?.num_sessions ?? 0,
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => {
          const count = params.row?.status?.num_sessions ?? 0;
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
  }, [theme.palette.link]);

  const columnGroupingModel = [
    {
      groupId: "statusGroup",
      headerName: "Status",
      children: [{ field: "registration" }, { field: "pduSessions" }],
    },
  ];

  const descriptionText =
    "Manage subscribers connecting to your private network. After creating a subscriber here, you can emit a SIM card with the corresponding IMSI, Key and OPc.";

  const knownCount = subscribersQuery.data?.total_count;

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <Box sx={{ mb: 3, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="h4" component="h1">
          {knownCount === undefined
            ? "Subscribers"
            : `Subscribers (${knownCount})`}
        </Typography>
        <Typography variant="body1" color="textSecondary">
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

      <QueryState
        query={subscribersQuery}
        resource="subscribers"
        isEmpty={(data) => (data.total_count ?? 0) === 0}
        empty={
          <EmptyState
            primaryText="No subscribers yet"
            secondaryText={
              canEdit
                ? "Create a subscriber to get started."
                : "Ask an administrator to create a subscriber."
            }
          />
        }
      >
        {(data) => (
          <DataGrid<APISubscriberSummary>
            rows={data.items ?? []}
            columns={columns}
            getRowId={(row) => row.imsi}
            columnGroupingModel={columnGroupingModel}
            disableRowSelectionOnClick
            paginationMode="server"
            rowCount={data.total_count ?? 0}
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
        )}
      </QueryState>

      {isCreateModalOpen && (
        <CreateSubscriberModal
          open
          onClose={() => setCreateModalOpen(false)}
          onSuccess={() => {
            void subscribersQuery.refetch();
            showSnackbar("Subscriber created successfully.", "success");
          }}
        />
      )}
    </Box>
  );
};

export default SubscriberPage;
