// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useState, type ReactNode } from "react";
import {
  Box,
  Typography,
  Chip,
  Stack,
  IconButton,
  Tooltip,
} from "@mui/material";
import { Edit as EditIcon } from "@mui/icons-material";
import { useQuery } from "@tanstack/react-query";
import { getInterfaces, type InterfacesInfo } from "@/queries/interfaces";
import { getStatus } from "@/queries/status";
import EditInterfaceN3Modal from "@/components/EditInterfaceN3Modal";
import NetworkTopology, {
  type InterfaceSegment,
} from "@/components/NetworkTopology";
import QueryState from "@/components/QueryState";
import { useNetworkingContext } from "./types";

function AddressLines({ addresses }: { addresses?: string[] }) {
  if (!addresses || addresses.length === 0) {
    return (
      <Typography variant="body2" color="textSecondary">
        Address: <strong>—</strong>
      </Typography>
    );
  }

  return addresses.map((address) => (
    <Typography key={address} variant="body2" color="textSecondary">
      Address: <strong>{address}</strong>
    </Typography>
  ));
}

function VlanLine({
  vlan,
}: {
  vlan?: { vlan_id?: number; master_interface?: string };
}) {
  if (!vlan) return null;

  return (
    <Typography variant="body2" color="textSecondary">
      VLAN:{" "}
      <strong>
        {vlan.vlan_id ?? "—"}
        {vlan.master_interface ? ` on ${vlan.master_interface}` : ""}
      </strong>
    </Typography>
  );
}

type InterfaceCardProps = {
  id: InterfaceSegment;
  title: string;
  chip: string;
  active: InterfaceSegment | null;
  onActiveChange: (segment: InterfaceSegment | null) => void;
  action?: ReactNode;
  children: ReactNode;
};

function InterfaceCard({
  id,
  title,
  chip,
  active,
  onActiveChange,
  action,
  children,
}: InterfaceCardProps) {
  const highlighted = active === id;

  return (
    <Box
      onMouseEnter={() => onActiveChange(id)}
      onMouseLeave={() => onActiveChange(null)}
      onFocus={() => onActiveChange(id)}
      onBlur={() => onActiveChange(null)}
      sx={{
        border: 1,
        borderColor: highlighted ? "primary.main" : "divider",
        boxShadow: highlighted ? 3 : 0,
        borderRadius: 2,
        p: 2,
        transition: (theme) =>
          theme.transitions.create(["border-color", "box-shadow"], {
            duration: 200,
          }),
      }}
    >
      <Stack
        direction="row"
        spacing={1}
        sx={{
          alignItems: "center",
          justifyContent: "space-between",
          mb: 1,
        }}
      >
        <Typography variant="subtitle1">{title}</Typography>
        <Stack direction="row" spacing={0.5} sx={{ alignItems: "center" }}>
          <Chip label={chip} size="small" />
          {action}
        </Stack>
      </Stack>
      {children}
    </Box>
  );
}

export default function InterfacesTab() {
  const { accessToken, canEdit, showSnackbar } = useNetworkingContext();
  const interfacesQuery = useQuery<InterfacesInfo>({
    queryKey: ["interfaces"],
    queryFn: () => getInterfaces(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
  });

  const statusQuery = useQuery({
    queryKey: ["status"],
    queryFn: getStatus,
  });

  const [isEditN3Open, setEditN3Open] = useState(false);
  const [active, setActive] = useState<InterfaceSegment | null>(null);

  const description =
    "View the network interfaces used by Ella Core for control plane (N2), user plane (N3), external networks (N6), and the API endpoint. Interfaces are primarily configured in the Ella Core configuration file; this page reflects that configuration, with N3's external address as the only editable field.";

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      <Box sx={{ mb: 2 }}>
        <Typography variant="h5" component="h2" sx={{ mb: 0.5 }}>
          Network Interfaces
        </Typography>
        <Typography variant="body2" color="textSecondary">
          {description}
        </Typography>
      </Box>

      <QueryState query={interfacesQuery} resource="network interfaces">
        {(interfacesInfo) => (
          <>
            <NetworkTopology
              interfaces={interfacesInfo}
              datapathAttachMode={statusQuery.data?.datapathAttachMode}
              active={active}
              onActiveChange={setActive}
            />

            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
                gap: 2,
                mt: 3,
              }}
            >
              <InterfaceCard
                id="n2"
                title="N2 (NGAP / S1AP)"
                chip="Control Plane"
                active={active}
                onActiveChange={setActive}
              >
                <AddressLines addresses={interfacesInfo.n2?.addresses} />
                <Typography variant="body2" color="textSecondary">
                  Port: <strong>{interfacesInfo.n2?.port ?? "—"}</strong>
                </Typography>
              </InterfaceCard>

              <InterfaceCard
                id="n3"
                title="N3 (GTP-U)"
                chip="User Plane"
                active={active}
                onActiveChange={setActive}
                action={
                  canEdit && (
                    <Tooltip title="Edit external address">
                      <IconButton
                        size="small"
                        onClick={() => setEditN3Open(true)}
                        color="primary"
                      >
                        <EditIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  )
                }
              >
                <Typography variant="body2" color="textSecondary">
                  Interface name:{" "}
                  <strong>{interfacesInfo.n3?.name ?? "—"}</strong>
                </Typography>
                <AddressLines addresses={interfacesInfo.n3?.addresses} />
                <Typography variant="body2" color="textSecondary">
                  External address:{" "}
                  <strong>{interfacesInfo.n3?.external_address || "—"}</strong>
                </Typography>
                <VlanLine vlan={interfacesInfo.n3?.vlan} />
              </InterfaceCard>

              <InterfaceCard
                id="n6"
                title="N6 (External)"
                chip="External Network"
                active={active}
                onActiveChange={setActive}
              >
                <Typography variant="body2" color="textSecondary">
                  Interface name:{" "}
                  <strong>{interfacesInfo.n6?.name ?? "—"}</strong>
                </Typography>
                <AddressLines addresses={interfacesInfo.n6?.addresses} />
                <VlanLine vlan={interfacesInfo.n6?.vlan} />
              </InterfaceCard>

              <InterfaceCard
                id="api"
                title="API"
                chip="Management"
                active={active}
                onActiveChange={setActive}
              >
                <AddressLines addresses={interfacesInfo.api?.addresses} />
                <Typography variant="body2" color="textSecondary">
                  Port: <strong>{interfacesInfo.api?.port ?? "—"}</strong>
                </Typography>
              </InterfaceCard>
            </Box>
          </>
        )}
      </QueryState>

      {isEditN3Open && (
        <EditInterfaceN3Modal
          open
          onClose={() => setEditN3Open(false)}
          onSuccess={() => {
            showSnackbar(
              "N3 external address updated successfully.",
              "success",
            );
            void interfacesQuery.refetch();
          }}
          initialData={{
            externalAddress: interfacesQuery.data?.n3?.external_address ?? "",
          }}
        />
      )}
    </Box>
  );
}
