// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useState } from "react";
import {
  Box,
  Typography,
  Button,
  Chip,
  TextField,
  InputAdornment,
  IconButton,
} from "@mui/material";
import SearchIcon from "@mui/icons-material/Search";
import ClearIcon from "@mui/icons-material/Clear";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useTheme } from "@mui/material/styles";
import { GridColDef, GridRenderCellParams } from "@mui/x-data-grid";
import EntityGrid from "@/components/grid/EntityGrid";
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
import { useDebouncedValue } from "@/hooks/useDebouncedValue";
import { useFilteredPagination } from "@/hooks/useFilteredPagination";
import ListPageHeader from "@/components/ListPageHeader";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const SubscriberPage: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const theme = useTheme();
  const canEdit = role === "Admin" || role === "Network Manager";

  const [searchInput, setSearchInput] = useState("");
  const appliedSearch = useDebouncedValue(searchInput).trim();

  const [paginationModel, setPaginationModel] = useFilteredPagination({
    search: appliedSearch,
  });

  const [isCreateModalOpen, setCreateModalOpen] = useState(false);
  const { showSnackbar } = useSnackbar();

  const pageOneBased = paginationModel.page + 1;
  const perPage = paginationModel.pageSize;

  const subscribersQuery = useQuery({
    queryKey: ["subscribers", pageOneBased, perPage, appliedSearch],
    queryFn: (): Promise<ListSubscribersResponse> =>
      listSubscribers(accessToken || "", pageOneBased, perPage, appliedSearch),
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
          (row?.status?.radio_access_types ?? []).join(" "),
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => {
          const rats = params.row?.status?.radio_access_types ?? [];
          if (rats.length === 0) return "—";
          return (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                height: "100%",
                gap: 0.5,
              }}
            >
              {rats.map((rat) => (
                <AccessChip key={rat} label={rat} />
              ))}
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
      children: [
        { field: "registration" },
        { field: "access" },
        { field: "sessions" },
      ],
    },
  ];

  const descriptionText =
    "Manage subscribers connecting to your private network. After creating a subscriber here, you can emit a SIM card with the corresponding IMSI, Key and OPc.";

  const knownCount = subscribersQuery.data?.total_count;

  // Nothing to search on a network with no subscribers, where the empty state
  // is the whole page. It stays put once a search is active so a zero-result
  // search can still be cleared.
  const showSearch = appliedSearch !== "" || (knownCount ?? 0) > 0;

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <ListPageHeader
        title="Subscribers"
        count={knownCount}
        description={descriptionText}
        filters={
          showSearch && (
            <TextField
              label="Search"
              type="search"
              placeholder="IMSI"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              size="small"
              sx={{ minWidth: 260 }}
              slotProps={{
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <SearchIcon fontSize="small" />
                    </InputAdornment>
                  ),
                  endAdornment: searchInput ? (
                    <InputAdornment position="end">
                      <IconButton
                        aria-label="Clear search"
                        size="small"
                        edge="end"
                        onClick={() => setSearchInput("")}
                      >
                        <ClearIcon fontSize="small" />
                      </IconButton>
                    </InputAdornment>
                  ) : null,
                },
              }}
            />
          )
        }
        action={
          canEdit && (
            <Button
              variant="contained"
              color="success"
              onClick={() => setCreateModalOpen(true)}
            >
              Create
            </Button>
          )
        }
      />

      <QueryState
        query={subscribersQuery}
        resource="subscribers"
        isEmpty={(data) => (data.total_count ?? 0) === 0}
        filtered={appliedSearch !== ""}
        noResults={
          <EmptyState
            primaryText="No subscribers match your search"
            secondaryText="Check the IMSI, or clear the search to see every subscriber."
          />
        }
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
          <EntityGrid<APISubscriberSummary>
            rows={data.items ?? []}
            columns={columns}
            getRowId={(row) => row.imsi}
            columnGroupingModel={columnGroupingModel}
            paginationMode="server"
            rowCount={data.total_count ?? 0}
            paginationModel={paginationModel}
            onPaginationModelChange={setPaginationModel}
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
