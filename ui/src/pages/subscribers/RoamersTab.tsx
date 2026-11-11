// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useState } from "react";
import {
  Box,
  Typography,
  TextField,
  InputAdornment,
  IconButton,
  Tooltip,
} from "@mui/material";
import SearchIcon from "@mui/icons-material/Search";
import ClearIcon from "@mui/icons-material/Clear";
import { Logout as DeregisterIcon } from "@mui/icons-material";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useTheme } from "@mui/material/styles";
import {
  GridActionsCellItem,
  GridColDef,
  GridRenderCellParams,
} from "@mui/x-data-grid";
import EntityGrid from "@/components/grid/EntityGrid";
import { Link } from "react-router-dom";
import {
  listRoamers,
  type APIRoamerSummary,
  type ListRoamersResponse,
} from "@/queries/roamers";
import DeregisterRoamerModal from "@/components/DeregisterRoamerModal";
import EmptyState from "@/components/EmptyState";
import QueryState from "@/components/QueryState";
import SubscriberStatusCell, {
  subscriberStatus,
} from "@/components/SubscriberStatusCell";
import { useAuth } from "@/contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";
import { useDebouncedState } from "@/hooks/useDebouncedState";
import { useFilteredPagination } from "@/hooks/useFilteredPagination";
import ListPageHeader from "@/components/ListPageHeader";

const MAX_SEARCH_LENGTH = 254;

type RoamersPage = ListRoamersResponse & { search: string };

const RoamersTab: React.FC = () => {
  const { role, accessToken, authReady } = useAuth();
  const theme = useTheme();
  const canEdit = role === "Admin" || role === "Network Manager";
  const { showSnackbar } = useSnackbar();

  const [searchInput, debouncedSearch, setSearchInput] = useDebouncedState();
  const appliedSearch = debouncedSearch.trim();

  const [paginationModel, setPaginationModel] = useFilteredPagination({
    search: appliedSearch,
  });

  const [roamerToDeregister, setRoamerToDeregister] = useState<string | null>(
    null,
  );

  const pageOneBased = paginationModel.page + 1;
  const perPage = paginationModel.pageSize;

  const roamersQuery = useQuery({
    queryKey: ["roamers", pageOneBased, perPage, appliedSearch],
    queryFn: async (): Promise<RoamersPage> => ({
      ...(await listRoamers(
        accessToken || "",
        pageOneBased,
        perPage,
        appliedSearch,
      )),
      search: appliedSearch,
    }),
    enabled: authReady && !!accessToken,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    retry: false,
    placeholderData: (prev) => prev,
  });

  const columns: GridColDef<APIRoamerSummary>[] = useMemo(() => {
    const base: GridColDef<APIRoamerSummary>[] = [
      {
        field: "imsi",
        headerName: "IMSI",
        flex: 1,
        minWidth: 150,
        renderCell: (params: GridRenderCellParams<APIRoamerSummary>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Link
              to={`/subscribers/roamers/${params.row.imsi}`}
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
        field: "partner",
        headerName: "Partner",
        flex: 1,
        minWidth: 130,
      },
      {
        field: "status",
        headerName: "Status",
        flex: 0.9,
        minWidth: 170,
        valueGetter: (_v, row: APIRoamerSummary) =>
          subscriberStatus(row?.status).rank,
        sortComparator: (v1, v2) => Number(v1) - Number(v2),
        renderCell: (params: GridRenderCellParams<APIRoamerSummary>) => (
          <SubscriberStatusCell status={params.row?.status} />
        ),
      },
      {
        field: "num_sessions",
        headerName: "Sessions",
        type: "number",
        flex: 0.5,
        minWidth: 100,
        valueGetter: (_v, row: APIRoamerSummary) =>
          row?.status?.num_sessions ?? 0,
      },
    ];

    if (canEdit) {
      base.push({
        field: "actions",
        headerName: "Actions",
        type: "actions",
        width: 80,
        getActions: (params) => [
          <GridActionsCellItem
            key="deregister"
            icon={
              <Tooltip title="Deregister" arrow>
                <DeregisterIcon color="primary" />
              </Tooltip>
            }
            label={`Deregister roamer ${params.row.imsi}`}
            onClick={() => setRoamerToDeregister(params.row.imsi)}
          />,
        ],
      });
    }

    return base;
  }, [theme.palette.link, canEdit]);

  const knownCount = roamersQuery.data?.total_count;

  const [hasRoamers, setHasRoamers] = useState(false);

  const unfilteredHasRows =
    roamersQuery.data?.search === ""
      ? (roamersQuery.data.total_count ?? 0) > 0
      : null;

  if (unfilteredHasRows !== null && unfilteredHasRows !== hasRoamers) {
    setHasRoamers(unfilteredHasRows);
  }

  const showSearch = appliedSearch !== "" || hasRoamers;

  return (
    <Box sx={{ mt: 3 }}>
      <ListPageHeader
        title="Roamers"
        count={knownCount}
        description="Subscribers of roaming partners registered on your network. Deregister a roamer to release it; it may register again."
        variant="section"
        filters={
          showSearch && (
            <TextField
              label="Search"
              type="search"
              placeholder="IMSI or MSISDN"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              size="small"
              sx={{
                minWidth: 260,
                '& input[type="search"]::-webkit-search-cancel-button': {
                  display: "none",
                },
              }}
              slotProps={{
                htmlInput: { maxLength: MAX_SEARCH_LENGTH },
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
      />

      <QueryState
        query={roamersQuery}
        resource="roamers"
        isEmpty={(data) => (data.total_count ?? 0) === 0}
        filtered={appliedSearch !== ""}
        noResults={
          <EmptyState
            primaryText="No roamers match your search"
            secondaryText="Check the IMSI, or clear the search to see every roamer."
          />
        }
        empty={
          <EmptyState
            primaryText="No roamers"
            secondaryText="No subscriber of a roaming partner is registered."
          />
        }
      >
        {(data) => (
          <EntityGrid<APIRoamerSummary>
            rows={data.items ?? []}
            columns={columns}
            getRowId={(row) => row.imsi}
            paginationMode="server"
            rowCount={data.total_count ?? 0}
            paginationModel={paginationModel}
            onPaginationModelChange={setPaginationModel}
          />
        )}
      </QueryState>

      {roamerToDeregister && (
        <DeregisterRoamerModal
          open
          onClose={() => setRoamerToDeregister(null)}
          onSuccess={() => {
            void roamersQuery.refetch();
            showSnackbar(
              `Roamer "${roamerToDeregister}" deregistered successfully.`,
              "success",
            );
          }}
          imsi={roamerToDeregister}
        />
      )}
    </Box>
  );
};

export default RoamersTab;
