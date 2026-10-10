// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useState, useMemo } from "react";
import { Box, Typography, Button, Chip } from "@mui/material";
import { useTheme } from "@mui/material/styles";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import {
  type GridColDef,
  type GridRenderCellParams,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import EntityGrid from "@/components/grid/EntityGrid";
import {
  listDataNetworks,
  type ListDataNetworksResponse,
  type APIDataNetwork,
} from "@/queries/data_networks";
import CreateDataNetworkModal from "@/components/CreateDataNetworkModal";
import EmptyState from "@/components/EmptyState";
import QueryState from "@/components/QueryState";
import ListPageHeader from "@/components/ListPageHeader";
import { useNetworkingContext } from "./types";

export default function DataNetworksTab() {
  const { accessToken, canEdit, showSnackbar } = useNetworkingContext();
  const outerTheme = useTheme();

  const [pagination, setPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const dataNetworksQuery = useQuery<ListDataNetworksResponse>({
    queryKey: ["data-networks", pagination.page, pagination.pageSize],
    queryFn: () =>
      listDataNetworks(
        accessToken || "",
        pagination.page + 1,
        pagination.pageSize,
      ),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    retry: false,
    placeholderData: (prev) => prev,
  });

  const [isCreateOpen, setCreateOpen] = useState(false);

  const description =
    "Manage the IP networks used by your subscribers. Data Network Names (DNNs) are used to identify different networks, and must be configured on the subscriber device to connect to the correct network.";

  const columns: GridColDef<APIDataNetwork>[] = useMemo(() => {
    return [
      {
        field: "name",
        headerName: "Name",
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
      { field: "ipv4_pool", headerName: "IPv4 Pool", flex: 1, minWidth: 180 },
      {
        field: "ipv6_pool",
        headerName: "IPv6 Pool",
        flex: 1,
        minWidth: 180,
        renderCell: (params: GridRenderCellParams<APIDataNetwork>) => (
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
              sx={params.row.ipv6_pool ? {} : { color: "text.secondary" }}
            >
              {params.row.ipv6_pool || "—"}
            </Typography>
          </Box>
        ),
      },
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

  const knownCount = dataNetworksQuery.data?.total_count;

  return (
    <Box
      sx={{
        width: "100%",
        mt: 2,
      }}
    >
      <ListPageHeader
        title="Data Networks"
        count={knownCount}
        description={description}
        variant="section"
        action={
          canEdit && (
            <Button
              variant="contained"
              color="success"
              onClick={() => setCreateOpen(true)}
            >
              Create
            </Button>
          )
        }
      />

      <QueryState
        query={dataNetworksQuery}
        resource="data networks"
        isEmpty={(data) => (data.total_count ?? 0) === 0}
        empty={
          <EmptyState
            primaryText="No data networks yet"
            secondaryText={
              canEdit
                ? "Create a data network to assign subscribers and control egress."
                : "Ask an administrator to create a data network."
            }
          />
        }
      >
        {(data) => (
          <EntityGrid<APIDataNetwork>
            rows={data.items ?? []}
            columns={columns}
            getRowId={(row) => row.name}
            paginationMode="server"
            rowCount={data.total_count ?? 0}
            paginationModel={pagination}
            onPaginationModelChange={setPagination}
          />
        )}
      </QueryState>

      {isCreateOpen && (
        <CreateDataNetworkModal
          open
          onClose={() => setCreateOpen(false)}
          onSuccess={() => {
            void dataNetworksQuery.refetch();
            showSnackbar("Data network created successfully.", "success");
          }}
        />
      )}
    </Box>
  );
}
