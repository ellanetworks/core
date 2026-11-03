// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useMemo, useState } from "react";
import { Box, Button, Typography } from "@mui/material";
import { Delete as DeleteIcon, Edit as EditIcon } from "@mui/icons-material";
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
import DeleteConfirmationModal from "@/components/DeleteConfirmationModal";
import RoamingDiameterPeerModal from "@/components/RoamingDiameterPeerModal";
import {
  deleteRoamingDiameterPeer,
  formatEndpoint,
  listRoamingDiameterPeers,
  listRoamingPartners,
  type RoamingDiameterPeer,
  type RoamingPartner,
} from "@/queries/roaming";
import { useRoamingContext } from "./types";

const cellSx = {
  display: "flex",
  alignItems: "center",
  width: "100%",
  height: "100%",
} as const;

export default function DiameterPeersTab() {
  const { accessToken, canEdit, showSnackbar } = useRoamingContext();
  const queryClient = useQueryClient();

  const [modal, setModal] = useState<{ peer?: RoamingDiameterPeer } | null>(
    null,
  );
  const [peerToDelete, setPeerToDelete] = useState<RoamingDiameterPeer | null>(
    null,
  );

  const peersQuery = useQuery<RoamingDiameterPeer[]>({
    queryKey: ["roaming-diameter-peers"],
    queryFn: () => listRoamingDiameterPeers(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const partnersQuery = useQuery<RoamingPartner[]>({
    queryKey: ["roaming-partners"],
    queryFn: () => listRoamingPartners(accessToken || ""),
    enabled: !!accessToken,
    placeholderData: (prev) => prev,
  });

  const usedBy = useMemo(() => {
    const map = new Map<string, string[]>();
    for (const partner of partnersQuery.data ?? []) {
      if (!partner.diameterPeerID) continue;
      map.set(partner.diameterPeerID, [
        ...(map.get(partner.diameterPeerID) ?? []),
        partner.name,
      ]);
    }
    return map;
  }, [partnersQuery.data]);

  const handleDelete = async () => {
    if (!peerToDelete || !accessToken) return;
    try {
      await deleteRoamingDiameterPeer(accessToken, peerToDelete.id);
      queryClient.invalidateQueries({ queryKey: ["roaming-diameter-peers"] });
      showSnackbar("Diameter peer deleted successfully.", "success");
    } catch (error) {
      const msg =
        error instanceof Error ? error.message : "Unknown error occurred.";
      showSnackbar(`Failed to delete Diameter peer: ${msg}`, "error");
    } finally {
      setPeerToDelete(null);
    }
  };

  const columns: GridColDef<RoamingDiameterPeer>[] = useMemo(
    () => [
      {
        field: "address",
        headerName: "Address",
        flex: 1,
        minWidth: 180,
        valueGetter: (_v, row) => formatEndpoint(row.address, row.port),
      },
      {
        field: "diameterIdentity",
        headerName: "Diameter Identity",
        flex: 1.3,
        minWidth: 220,
        renderCell: (params: GridRenderCellParams<RoamingDiameterPeer>) => (
          <Box sx={cellSx}>
            {params.row.diameterIdentity ? (
              <Typography variant="body2" noWrap>
                {params.row.diameterIdentity}
              </Typography>
            ) : (
              <Typography variant="body2" color="textSecondary">
                Any
              </Typography>
            )}
          </Box>
        ),
      },
      {
        field: "partners",
        headerName: "Partners",
        flex: 1,
        minWidth: 160,
        valueGetter: (_v, row) => (usedBy.get(row.id) ?? []).join(", "),
        renderCell: (params: GridRenderCellParams<RoamingDiameterPeer>) => {
          const names = usedBy.get(params.row.id) ?? [];
          return (
            <Box sx={cellSx}>
              <Typography
                variant="body2"
                noWrap
                color={names.length ? undefined : "textSecondary"}
              >
                {names.length ? names.join(", ") : "—"}
              </Typography>
            </Box>
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
              getActions: (p: { row: RoamingDiameterPeer }) => {
                const endpoint = formatEndpoint(p.row.address, p.row.port);
                const used = (usedBy.get(p.row.id) ?? []).length > 0;
                return [
                  <GridActionsCellItem
                    key="edit"
                    icon={<EditIcon color="primary" />}
                    label={`Edit Diameter peer ${endpoint}`}
                    onClick={() => setModal({ peer: p.row })}
                  />,
                  <GridActionsCellItem
                    key="delete"
                    icon={<DeleteIcon color="primary" />}
                    label={
                      used
                        ? `Diameter peer ${endpoint} is used by a partner`
                        : `Delete Diameter peer ${endpoint}`
                    }
                    disabled={used}
                    onClick={() => setPeerToDelete(p.row)}
                  />,
                ];
              },
            } as GridColDef<RoamingDiameterPeer>,
          ]
        : []),
    ],
    [canEdit, usedBy],
  );

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      <ListPageHeader
        title="Diameter Peers"
        count={peersQuery.data?.length}
        description="The Diameter peers that reach the home HSS of 4G roamers: a partner's HSS or a Diameter Edge Agent shared by several partners. A peer in use by a partner cannot be deleted."
        variant="section"
        action={
          canEdit && (
            <Button
              variant="contained"
              color="success"
              onClick={() => setModal({})}
            >
              Add Peer
            </Button>
          )
        }
      />

      <QueryState
        query={peersQuery}
        resource="Diameter peers"
        isEmpty={(data) => data.length === 0}
        empty={
          <EmptyState
            primaryText="No Diameter peers yet"
            secondaryText={
              canEdit
                ? "Add a Diameter peer, then select it in a partner's 4G roaming settings."
                : "Ask an administrator to add a Diameter peer."
            }
          />
        }
      >
        {(data) => (
          <EntityGrid<RoamingDiameterPeer>
            rows={data}
            columns={columns}
            getRowId={(row) => row.id}
          />
        )}
      </QueryState>

      {modal && (
        <RoamingDiameterPeerModal
          open
          onClose={() => setModal(null)}
          onSuccess={() => {
            queryClient.invalidateQueries({
              queryKey: ["roaming-diameter-peers"],
            });
            showSnackbar(
              modal.peer
                ? "Diameter peer updated successfully."
                : "Diameter peer added successfully.",
              "success",
            );
          }}
          peer={modal.peer}
        />
      )}
      {peerToDelete && (
        <DeleteConfirmationModal
          open
          onClose={() => setPeerToDelete(null)}
          onConfirm={handleDelete}
          title="Delete Diameter Peer"
          description={`Delete the Diameter peer ${formatEndpoint(peerToDelete.address, peerToDelete.port)}? This cannot be undone.`}
        />
      )}
    </Box>
  );
}
