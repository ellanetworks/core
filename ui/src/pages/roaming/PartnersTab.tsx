// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React, { useCallback, useMemo, useState } from "react";
import { Box, Button, Tooltip, Typography } from "@mui/material";
import { useTheme } from "@mui/material/styles";
import {
  Delete as DeleteIcon,
  Edit as EditIcon,
  Visibility as ViewIcon,
} from "@mui/icons-material";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "react-router-dom";
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
import RoamingPartnerModal from "@/components/RoamingPartnerModal";
import ViewRoamingPartnerModal from "@/components/ViewRoamingPartnerModal";
import {
  deleteRoamingPartner,
  diameterPeerLabel,
  formatPLMN,
  getSEPPCertificate,
  listRoamingDiameterPeers,
  listRoamingPartners,
  type RoamingDiameterPeer,
  type RoamingPartner,
  type RoamingPartnerInput,
} from "@/queries/roaming";
import { useRoamingContext } from "./types";

const cellSx = {
  display: "flex",
  alignItems: "center",
  width: "100%",
  height: "100%",
} as const;

const Muted = () => (
  <Typography variant="body2" color="textSecondary">
    —
  </Typography>
);

const plmnList = (p: RoamingPartner) => p.plmns.map(formatPLMN).join(", ");

export default function PartnersTab() {
  const { accessToken, canEdit, showSnackbar } = useRoamingContext();
  const theme = useTheme();
  const queryClient = useQueryClient();

  const [modal, setModal] = useState<{ partner?: RoamingPartner } | null>(null);
  const [partnerToView, setPartnerToView] = useState<RoamingPartner | null>(
    null,
  );
  const [partnerToDelete, setPartnerToDelete] = useState<RoamingPartner | null>(
    null,
  );

  const partnersQuery = useQuery<RoamingPartner[]>({
    queryKey: ["roaming-partners"],
    queryFn: () => listRoamingPartners(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
    placeholderData: (prev) => prev,
  });

  const peersQuery = useQuery<RoamingDiameterPeer[]>({
    queryKey: ["roaming-diameter-peers"],
    queryFn: () => listRoamingDiameterPeers(accessToken || ""),
    enabled: !!accessToken,
    placeholderData: (prev) => prev,
  });

  const seppQuery = useQuery({
    queryKey: ["sepp-certificate"],
    queryFn: () => getSEPPCertificate(accessToken || ""),
    enabled: !!accessToken && modal !== null,
  });

  const peers = useMemo(() => peersQuery.data ?? [], [peersQuery.data]);
  const peersByID = useMemo(
    () => new Map(peers.map((peer) => [peer.id, peer])),
    [peers],
  );

  const handleSaved = (input: RoamingPartnerInput) => {
    const generated = !!input.sepp && seppQuery.data === null;
    queryClient.invalidateQueries({ queryKey: ["roaming-partners"] });
    queryClient.invalidateQueries({ queryKey: ["sepp-certificate"] });
    const action = modal?.partner ? "updated" : "added";
    showSnackbar(
      generated
        ? `Roaming partner ${action}. A SEPP certificate was generated: give the partner its trust anchor from the SEPP tab.`
        : `Roaming partner ${action} successfully.`,
      "success",
    );
  };

  const handleDelete = async () => {
    if (!partnerToDelete || !accessToken) return;
    try {
      await deleteRoamingPartner(accessToken, partnerToDelete.id);
      queryClient.invalidateQueries({ queryKey: ["roaming-partners"] });
      queryClient.invalidateQueries({ queryKey: ["roaming-usage"] });
      showSnackbar(
        `Roaming partner "${partnerToDelete.name}" deleted successfully.`,
        "success",
      );
    } catch (error) {
      const msg =
        error instanceof Error ? error.message : "Unknown error occurred.";
      showSnackbar(`Failed to delete roaming partner: ${msg}`, "error");
    } finally {
      setPartnerToDelete(null);
    }
  };

  const openEdit = useCallback(
    (partner: RoamingPartner) => setModal({ partner }),
    [],
  );

  const columns: GridColDef<RoamingPartner>[] = useMemo(
    () => [
      { field: "name", headerName: "Name", flex: 1, minWidth: 160 },
      {
        field: "plmns",
        headerName: "PLMNs",
        flex: 0.8,
        minWidth: 110,
        valueGetter: (_v, row) => plmnList(row),
      },
      {
        field: "profileName",
        headerName: "Profile",
        flex: 1,
        minWidth: 140,
        renderCell: (params: GridRenderCellParams<RoamingPartner>) => (
          <Box sx={cellSx}>
            <Link
              to={`/profiles/${encodeURIComponent(params.row.profileName)}`}
              style={{ textDecoration: "none" }}
              onClick={(e: React.MouseEvent) => e.stopPropagation()}
            >
              <Typography
                variant="body2"
                sx={{
                  color: theme.palette.link,
                  textDecoration: "underline",
                }}
              >
                {params.row.profileName}
              </Typography>
            </Link>
          </Box>
        ),
      },
      {
        field: "sepp",
        headerName: "5G (SEPP)",
        flex: 1.3,
        minWidth: 200,
        valueGetter: (_v, row) => row.sepp?.fqdn ?? "",
        renderCell: (params: GridRenderCellParams<RoamingPartner>) => (
          <Box sx={cellSx}>
            {params.row.sepp ? (
              <Tooltip title={`N32-c ${params.row.sepp.n32cURI}`} arrow>
                <Typography variant="body2" noWrap>
                  {params.row.sepp.fqdn}
                </Typography>
              </Tooltip>
            ) : (
              <Muted />
            )}
          </Box>
        ),
      },
      {
        field: "diameterPeerID",
        headerName: "4G (Diameter Peer)",
        flex: 1.3,
        minWidth: 200,
        renderCell: (params: GridRenderCellParams<RoamingPartner>) => {
          const id = params.row.diameterPeerID;
          const peer = id ? peersByID.get(id) : undefined;
          return (
            <Box sx={cellSx}>
              {id ? (
                <Typography variant="body2" noWrap>
                  {peer ? diameterPeerLabel(peer) : id}
                </Typography>
              ) : (
                <Muted />
              )}
            </Box>
          );
        },
      },
      {
        field: "actions",
        headerName: "Actions",
        type: "actions",
        width: canEdit ? 140 : 70,
        getActions: (p: { row: RoamingPartner }) => [
          <GridActionsCellItem
            key="view"
            icon={<ViewIcon color="primary" />}
            label={`View roaming partner ${p.row.name}`}
            onClick={() => setPartnerToView(p.row)}
          />,
          ...(canEdit
            ? [
                <GridActionsCellItem
                  key="edit"
                  icon={<EditIcon color="primary" />}
                  label={`Edit roaming partner ${p.row.name}`}
                  onClick={() => openEdit(p.row)}
                />,
                <GridActionsCellItem
                  key="delete"
                  icon={<DeleteIcon color="primary" />}
                  label={`Delete roaming partner ${p.row.name}`}
                  onClick={() => setPartnerToDelete(p.row)}
                />,
              ]
            : []),
        ],
      } as GridColDef<RoamingPartner>,
    ],
    [canEdit, openEdit, peersByID, theme],
  );

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      <ListPageHeader
        title="Partners"
        count={partnersQuery.data?.length}
        description="Each partner is a roaming agreement with one home network, which may have several PLMN IDs. Its subscribers may attach over 5G through its SEPP, over 4G through a Diameter peer, or both."
        variant="section"
        action={
          canEdit && (
            <Button
              variant="contained"
              color="success"
              onClick={() => setModal({})}
            >
              Add Partner
            </Button>
          )
        }
      />

      <QueryState
        query={partnersQuery}
        resource="roaming partners"
        isEmpty={(data) => data.length === 0}
        empty={
          <EmptyState
            primaryText="No roaming partners yet"
            secondaryText={
              canEdit
                ? "Add a partner to let its subscribers attach to this network."
                : "Ask an administrator to add a roaming partner."
            }
          />
        }
      >
        {(data) => (
          <EntityGrid<RoamingPartner>
            rows={data}
            columns={columns}
            getRowId={(row) => row.id}
          />
        )}
      </QueryState>

      {modal && (
        <RoamingPartnerModal
          open
          onClose={() => setModal(null)}
          onSuccess={handleSaved}
          partner={modal.partner}
          diameterPeers={peers}
        />
      )}
      {partnerToView && (
        <ViewRoamingPartnerModal
          open
          onClose={() => setPartnerToView(null)}
          partner={partnerToView}
          diameterPeers={peers}
        />
      )}
      {partnerToDelete && (
        <DeleteConfirmationModal
          open
          onClose={() => setPartnerToDelete(null)}
          onConfirm={handleDelete}
          title="Delete Roaming Partner"
          description={`Delete the roaming partner "${partnerToDelete.name}" (PLMN ${plmnList(partnerToDelete)})? A partner with registered roamers cannot be deleted. Its roaming usage is kept. This cannot be undone.`}
        />
      )}
    </Box>
  );
}
