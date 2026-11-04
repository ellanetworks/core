// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useState } from "react";
import { Box, Chip, Typography } from "@mui/material";
import { useTheme } from "@mui/material/styles";
import {
  type GridColDef,
  type GridPaginationModel,
  type GridRenderCellParams,
} from "@mui/x-data-grid";
import { useQuery } from "@tanstack/react-query";
import { Link as RouterLink } from "react-router-dom";
import EntityGrid, { EMBEDDED_GRID_HEIGHT } from "@/components/grid/EntityGrid";
import QueryState from "@/components/QueryState";
import type { APIRoamerSummary, ListRoamersResponse } from "@/queries/roamers";
import { formatDateTime } from "@/utils/formatters";

type RoamersSectionProps = {
  title: string;
  queryKey: readonly unknown[];
  fetchPage: (page: number, perPage: number) => Promise<ListRoamersResponse>;
  enabled: boolean;
};

const RoamersSection: React.FC<RoamersSectionProps> = ({
  title,
  queryKey,
  fetchPage,
  enabled,
}) => {
  const theme = useTheme();

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 10,
  });

  const page = paginationModel.page + 1;
  const perPage = paginationModel.pageSize;

  const roamersQuery = useQuery({
    queryKey: [...queryKey, page, perPage],
    queryFn: () => fetchPage(page, perPage),
    enabled,
    refetchInterval: 5000,
    retry: false,
  });

  const columns: GridColDef<APIRoamerSummary>[] = useMemo(
    () => [
      {
        field: "imsi",
        headerName: "IMSI",
        flex: 1,
        minWidth: 140,
        renderCell: (params: GridRenderCellParams<APIRoamerSummary>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <RouterLink
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
            </RouterLink>
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
        field: "sessions",
        headerName: "Sessions",
        flex: 0.5,
        minWidth: 100,
        valueGetter: (_v, row: APIRoamerSummary) =>
          row?.status?.num_sessions ?? 0,
        renderCell: (params: GridRenderCellParams<APIRoamerSummary>) => {
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
      {
        field: "lastSeenAt",
        headerName: "Last Seen",
        flex: 0.8,
        minWidth: 120,
        valueGetter: (_v, row: APIRoamerSummary) =>
          row?.status?.last_seen_at ?? "",
        renderCell: (params: GridRenderCellParams<APIRoamerSummary>) => {
          const ts = params.row?.status?.last_seen_at;
          return (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                width: "100%",
                height: "100%",
              }}
            >
              <Typography
                variant="body2"
                sx={ts ? {} : { color: "text.secondary" }}
              >
                {ts ? formatDateTime(ts) : "—"}
              </Typography>
            </Box>
          );
        },
      },
    ],
    [theme],
  );

  const total = roamersQuery.data?.total_count ?? 0;

  if (total === 0 && !roamersQuery.isLoadingError) return null;

  return (
    <Box sx={{ mt: 3 }}>
      <Typography variant="h6" sx={{ mb: 1 }}>
        {roamersQuery.data ? `${title} (${total})` : title}
      </Typography>
      <QueryState query={roamersQuery} resource={title.toLowerCase()}>
        {(data) => (
          <EntityGrid<APIRoamerSummary>
            rows={data.items ?? []}
            columns={columns}
            getRowId={(row) => row.imsi}
            paginationMode="server"
            rowCount={data.total_count ?? 0}
            paginationModel={paginationModel}
            onPaginationModelChange={setPaginationModel}
            variant="embedded"
            height={EMBEDDED_GRID_HEIGHT}
          />
        )}
      </QueryState>
    </Box>
  );
};

export default RoamersSection;
