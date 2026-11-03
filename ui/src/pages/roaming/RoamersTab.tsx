// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useMemo, useState } from "react";
import { Alert, Box, Chip } from "@mui/material";
import {
  Logout as DeregisterIcon,
  Visibility as ViewIcon,
} from "@mui/icons-material";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  GridActionsCellItem,
  type GridColDef,
  type GridRenderCellParams,
} from "@mui/x-data-grid";
import EntityGrid from "@/components/grid/EntityGrid";
import EmptyState from "@/components/EmptyState";
import QueryState from "@/components/QueryState";
import ListPageHeader from "@/components/ListPageHeader";
import AccessChip from "@/components/AccessChip";
import ConfirmDialog from "@/components/form/ConfirmDialog";
import ViewRoamerModal from "@/components/ViewRoamerModal";
import {
  deregisterRoamer,
  listRoamers,
  type Roamer,
  type RoamersList,
} from "@/queries/roaming";
import { roamerRATs, roamerState } from "@/utils/roamers";
import { useRoamingContext } from "./types";

export default function RoamersTab() {
  const { accessToken, canEdit, showSnackbar } = useRoamingContext();
  const queryClient = useQueryClient();

  const [roamerToView, setRoamerToView] = useState<Roamer | null>(null);
  const [roamerToDeregister, setRoamerToDeregister] = useState<Roamer | null>(
    null,
  );

  const roamersQuery = useQuery<RoamersList>({
    queryKey: ["roaming-roamers"],
    queryFn: () => listRoamers(accessToken || ""),
    enabled: !!accessToken,
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
    retry: false,
    placeholderData: (prev) => prev,
  });

  const unreachable = roamersQuery.data?.unreachableNodes ?? [];

  const handleDeregister = async () => {
    if (!roamerToDeregister || !accessToken) return;
    const { imsi } = roamerToDeregister;
    await deregisterRoamer(accessToken, imsi);
    setRoamerToDeregister(null);
    queryClient.invalidateQueries({ queryKey: ["roaming-roamers"] });
    showSnackbar(`Roamer "${imsi}" deregistered successfully.`, "success");
  };

  const columns: GridColDef<Roamer>[] = useMemo(
    () => [
      { field: "imsi", headerName: "IMSI", flex: 1, minWidth: 150 },
      {
        field: "partnerName",
        headerName: "Partner",
        flex: 1,
        minWidth: 150,
      },
      {
        field: "sessions",
        headerName: "Sessions",
        width: 120,
        valueGetter: (_v, row) => row.sessions.length,
        renderCell: (params: GridRenderCellParams<Roamer>) => {
          const count = params.row.sessions.length;
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
      {
        field: "status",
        headerName: "Status",
        flex: 0.9,
        minWidth: 170,
        valueGetter: (_v, row) => roamerState(row.registrations).rank,
        sortComparator: (v1, v2) => Number(v1) - Number(v2),
        renderCell: (params: GridRenderCellParams<Roamer>) => {
          const { label, color } = roamerState(params.row.registrations);
          return (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                height: "100%",
                gap: 0.5,
              }}
            >
              <Chip size="small" label={label} color={color} variant="filled" />
              {roamerRATs(params.row.registrations).map((rat) => (
                <AccessChip key={rat} label={rat} />
              ))}
            </Box>
          );
        },
      },
      {
        field: "actions",
        headerName: "Actions",
        type: "actions",
        width: canEdit ? 100 : 60,
        disableColumnMenu: true,
        getActions: (p: { row: Roamer }) => [
          <GridActionsCellItem
            key="view"
            icon={<ViewIcon color="primary" />}
            label={`View roamer ${p.row.imsi}`}
            onClick={() => setRoamerToView(p.row)}
          />,
          ...(canEdit
            ? [
                <GridActionsCellItem
                  key="deregister"
                  icon={<DeregisterIcon color="primary" />}
                  label={`Deregister roamer ${p.row.imsi}`}
                  onClick={() => setRoamerToDeregister(p.row)}
                />,
              ]
            : []),
        ],
      } as GridColDef<Roamer>,
    ],
    [canEdit],
  );

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      <ListPageHeader
        title="Roamers"
        count={roamersQuery.data?.items.length}
        description="Subscribers of roaming partners currently registered in your network."
        variant="section"
      />

      {unreachable.length > 0 && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          {`Some nodes did not answer, so this list may be incomplete: ${unreachable.map((node) => node.nodeID).join(", ")}.`}
        </Alert>
      )}

      <QueryState
        query={roamersQuery}
        resource="roamers"
        isEmpty={(data) => data.items.length === 0}
        empty={
          <EmptyState
            primaryText="No roamers"
            secondaryText="No subscriber of a roaming partner is registered in your network."
          />
        }
      >
        {(data) => (
          <EntityGrid<Roamer>
            rows={data.items}
            columns={columns}
            getRowId={(row) => row.imsi}
          />
        )}
      </QueryState>

      {roamerToView && accessToken && (
        <ViewRoamerModal
          open
          onClose={() => setRoamerToView(null)}
          roamer={roamerToView}
          accessToken={accessToken}
        />
      )}
      {roamerToDeregister && (
        <ConfirmDialog
          open
          onClose={() => setRoamerToDeregister(null)}
          onConfirm={handleDeregister}
          title="Confirm Deregistration"
          description={`Are you sure you want to deregister the roamer "${roamerToDeregister.imsi}"?`}
          confirmLabel="Deregister"
          confirmingLabel="Deregistering…"
          confirmColor="error"
        />
      )}
    </Box>
  );
}
