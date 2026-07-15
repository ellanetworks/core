// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useMemo, useState } from "react";
import { Box, Typography } from "@mui/material";
import { useTheme } from "@mui/material/styles";
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
import QueryState from "@/components/QueryState";
import RanNodeTypeChip from "@/components/RanNodeTypeChip";
import { useAuth } from "@/contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";

export default function RadiosListTab() {
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
        field: "type",
        headerName: "Type",
        flex: 0.4,
        minWidth: 80,
        renderCell: (params) => <RanNodeTypeChip type={params.row.type} />,
      },
      { field: "address", headerName: "Address", flex: 1, minWidth: 120 },
    ],
    [theme],
  );

  const descriptionText =
    "View connected radios and their network locations. Radios will automatically appear here once connected.";

  return (
    <>
      <Box
        sx={{
          width: "100%",
          mb: 3,
          display: "flex",
          flexDirection: "column",
          gap: 2,
          mt: 2,
        }}
      >
        <Typography variant="h4" component="h1">
          {knownCount === undefined ? "Radios" : `Radios (${knownCount})`}
        </Typography>
        <Typography variant="body1" color="textSecondary">
          {descriptionText}
        </Typography>
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
            <DataGrid<APIRadio>
              rows={data.items ?? []}
              columns={columns}
              getRowId={(row) => row.address}
              paginationMode="server"
              rowCount={data.total_count ?? 0}
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
          </Box>
        )}
      </QueryState>
    </>
  );
}
