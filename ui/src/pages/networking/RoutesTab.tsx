// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useState, useMemo } from "react";
import { Box, Typography, Button, Chip } from "@mui/material";
import { Delete as DeleteIcon } from "@mui/icons-material";
import { useQuery } from "@tanstack/react-query";
import {
  DataGrid,
  type GridColDef,
  GridActionsCellItem,
  type GridRenderCellParams,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import {
  listRoutes,
  deleteRoute,
  type ListRoutesResponse,
  type APIRoute,
} from "@/queries/routes";
import CreateRouteModal from "@/components/CreateRouteModal";
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import EmptyState from "@/components/EmptyState";
import QueryState from "@/components/QueryState";
import { useNetworkingContext } from "./types";

export default function RoutesTab() {
  const { accessToken, canEdit, showSnackbar } = useNetworkingContext();
  const [pagination, setPagination] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const routesQuery = useQuery<ListRoutesResponse>({
    queryKey: ["routes", pagination.page, pagination.pageSize],
    queryFn: () =>
      listRoutes(accessToken || "", pagination.page + 1, pagination.pageSize),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const refetch = () => void routesQuery.refetch();

  const [isCreateOpen, setCreateOpen] = useState(false);
  const [isDeleteOpen, setDeleteOpen] = useState(false);
  const [selectedRouteId, setSelectedRouteId] = useState<number | null>(null);

  const handleRequestDelete = (routeID: number) => {
    setSelectedRouteId(routeID);
    setDeleteOpen(true);
  };

  const handleConfirmDelete = async () => {
    if (selectedRouteId == null || !accessToken) return;
    try {
      await deleteRoute(accessToken, selectedRouteId);
      setDeleteOpen(false);
      showSnackbar(
        `Route "${selectedRouteId}" deleted successfully.`,
        "success",
      );
      refetch();
    } catch (error: unknown) {
      setDeleteOpen(false);
      showSnackbar(
        `Failed to delete route "${selectedRouteId}": ${String(error)}`,
        "error",
      );
    } finally {
      setSelectedRouteId(null);
    }
  };

  const description =
    "Manage the routing table for subscriber traffic. Created routes are applied as kernel routes on the node running Ella Core.";

  const columns: GridColDef<APIRoute>[] = useMemo(() => {
    return [
      { field: "id", headerName: "ID", width: 70 },
      {
        field: "destination",
        headerName: "Destination",
        flex: 1,
        minWidth: 120,
      },
      { field: "gateway", headerName: "Gateway", flex: 1, minWidth: 120 },
      { field: "interface", headerName: "Interface", flex: 1, minWidth: 100 },
      { field: "metric", headerName: "Metric", width: 80 },
      {
        field: "source",
        headerName: "Source",
        flex: 0.5,
        minWidth: 80,
        renderCell: (params: GridRenderCellParams<APIRoute>) => {
          const source = params.row.source;
          return (
            <Chip
              size="small"
              label={source === "bgp" ? "BGP" : "Static"}
              color={source === "bgp" ? "info" : "default"}
              variant="outlined"
            />
          );
        },
      },
      ...(canEdit
        ? [
            {
              field: "actions",
              headerName: "Actions",
              type: "actions",
              width: 100,
              sortable: false,
              disableColumnMenu: true,
              getActions: (p: { row: APIRoute }) =>
                p.row.source === "bgp"
                  ? []
                  : [
                      <GridActionsCellItem
                        key="delete"
                        icon={<DeleteIcon color="primary" />}
                        label="Delete"
                        onClick={() => handleRequestDelete(p.row.id)}
                      />,
                    ],
            } as GridColDef<APIRoute>,
          ]
        : []),
    ];
  }, [canEdit]);

  const knownCount = routesQuery.data?.total_count;

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      <Box sx={{ mb: 3 }}>
        <Typography variant="h5" sx={{ mb: 0.5 }}>
          {knownCount === undefined ? "Routes" : `Routes (${knownCount})`}
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

      <QueryState
        query={routesQuery}
        resource="routes"
        isEmpty={(data) => (data.total_count ?? 0) === 0}
        empty={
          <EmptyState
            primaryText="No routes yet"
            secondaryText={
              canEdit
                ? "Create a route so UEs can reach external networks."
                : "Ask an administrator to create a route."
            }
          />
        }
      >
        {(data) => (
          <DataGrid<APIRoute>
            rows={data.items ?? []}
            columns={columns}
            getRowId={(row) =>
              row.source === "bgp"
                ? `bgp-${row.destination}-${row.gateway}`
                : row.id
            }
            paginationMode="server"
            rowCount={data.total_count ?? 0}
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
        )}
      </QueryState>

      {isCreateOpen && (
        <CreateRouteModal
          open
          onClose={() => setCreateOpen(false)}
          onSuccess={() => {
            refetch();
            showSnackbar("Route created successfully.", "success");
          }}
        />
      )}
      {isDeleteOpen && (
        <DeleteConfirmationModal
          open
          onClose={() => setDeleteOpen(false)}
          onConfirm={handleConfirmDelete}
          title="Confirm Deletion"
          description={`Are you sure you want to delete the route "${selectedRouteId}"? This action cannot be undone.`}
        />
      )}
    </Box>
  );
}
