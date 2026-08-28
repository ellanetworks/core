// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useState } from "react";
import { Box, Typography } from "@mui/material";
import { useQuery } from "@tanstack/react-query";
import { getInterfaces, type InterfacesInfo } from "@/queries/interfaces";
import { getStatus } from "@/queries/status";
import EditInterfaceN3Modal from "@/components/EditInterfaceN3Modal";
import NetworkTopology from "@/components/NetworkTopology";
import QueryState from "@/components/QueryState";
import { useNetworkingContext } from "./types";

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

  const description =
    "The network interfaces Ella Core runs on: control plane (N2), user plane (N3), external network (N6), and the API endpoint. Interfaces are configured in the Ella Core configuration file; this page reflects that configuration, with N3's external address as the only editable field.";

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
          <NetworkTopology
            interfaces={interfacesInfo}
            datapathAttachMode={statusQuery.data?.datapathAttachMode}
            canEdit={canEdit}
            onEditN3={() => setEditN3Open(true)}
          />
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
