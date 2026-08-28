// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useState } from "react";
import { Box, Button, Typography } from "@mui/material";
import { useTheme } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import { type GridColDef, type GridPaginationModel } from "@mui/x-data-grid";
import EntityGrid from "@/components/grid/EntityGrid";
import { Link, Link as RouterLink } from "react-router-dom";

import {
  listRadios,
  radioPath,
  type APIRadio,
  type ListRadiosResponse,
} from "@/queries/radios";
import EmptyState from "@/components/EmptyState";
import QueryState from "@/components/QueryState";
import RadioStatusChip from "@/components/RadioStatusChip";
import RanNodeTypeChip from "@/components/RanNodeTypeChip";
import { useAuth } from "@/contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

export default function RadiosList() {
  const { accessToken } = useAuth();
  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });
  const radiosQuery = useQuery<ListRadiosResponse>({
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
    refetchOnWindowFocus: true,
    retry: false,
    placeholderData: (prev) => prev,
  });

  const knownCount = radiosQuery.data?.total_count;

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
              to={`/radios/${radioPath({ type: params.row.type, id: params.row.id })}`}
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
                {params.row.name || `${params.row.type} ${params.row.id}`}
              </Typography>
            </Link>
          </Box>
        ),
      },
      { field: "id", headerName: "ID", flex: 0.6, minWidth: 100 },
      {
        field: "type",
        headerName: "Type",
        flex: 0.4,
        minWidth: 80,
        renderCell: (params) => <RanNodeTypeChip type={params.row.type} />,
      },
      {
        field: "status",
        headerName: "Status",
        flex: 0.4,
        minWidth: 90,
        renderCell: (params) => <RadioStatusChip status={params.row.status} />,
      },
      { field: "address", headerName: "Address", flex: 1, minWidth: 120 },
    ],
    [theme],
  );

  const descriptionText =
    "View radios and their network locations. Radios will automatically appear here once connected.";

  return (
    <Box
      sx={{ pt: 6, pb: 4, maxWidth: MAX_WIDTH, mx: "auto", px: PAGE_PADDING_X }}
    >
      <Box
        sx={{
          display: "flex",
          flexDirection: { xs: "column", sm: "row" },
          alignItems: { xs: "flex-start", sm: "center" },
          gap: 2,
          mb: 3,
        }}
      >
        <Box sx={{ flex: 1 }}>
          <Typography variant="h4" component="h1" sx={{ mb: 1 }}>
            {knownCount === undefined ? "Radios" : `Radios (${knownCount})`}
          </Typography>
          <Typography variant="body1" color="textSecondary">
            {descriptionText}
          </Typography>
        </Box>
        <Button component={RouterLink} to="/radios/events" variant="outlined">
          Network events
        </Button>
      </Box>

      <QueryState
        query={radiosQuery}
        resource="radios"
        isEmpty={(data) => (data.total_count ?? 0) === 0}
        empty={
          <EmptyState
            primaryText="No radios yet"
            secondaryText="Connected radios will automatically appear here."
          />
        }
      >
        {(data) => (
          <Box sx={{ width: "100%" }}>
            <EntityGrid<APIRadio>
              rows={data.items ?? []}
              columns={columns}
              getRowId={(row) => `${row.type}:${row.id || row.address}`}
              paginationMode="server"
              rowCount={data.total_count ?? 0}
              paginationModel={paginationModel}
              onPaginationModelChange={setPaginationModel}
              columnVisibilityModel={{ id: !isSmDown }}
            />
          </Box>
        )}
      </QueryState>
    </Box>
  );
}
