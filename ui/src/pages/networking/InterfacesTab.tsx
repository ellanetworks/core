import { useState } from "react";
import {
  Box,
  Typography,
  CircularProgress,
  Chip,
  Stack,
  IconButton,
  Tooltip,
} from "@mui/material";
import { Edit as EditIcon } from "@mui/icons-material";
import { useQuery } from "@tanstack/react-query";
import { getInterfaces, type InterfacesInfo } from "@/queries/interfaces";
import EditInterfaceN3Modal from "@/components/EditInterfaceN3Modal";
import EmptyState from "@/components/EmptyState";
import { useNetworkingContext } from "./types";

export default function InterfacesTab() {
  const { accessToken, canEdit, showSnackbar } = useNetworkingContext();
  const {
    data: interfacesInfo,
    isLoading: loading,
    refetch,
  } = useQuery<InterfacesInfo>({
    queryKey: ["interfaces"],
    queryFn: () => getInterfaces(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
  });

  const [isEditN3Open, setEditN3Open] = useState(false);

  const description =
    "View the network interfaces used by Ella Core for control plane (N2), user plane (N3), external networks (N6), and the API endpoint. Interfaces are primarily configured in the Ella Core configuration file; this page reflects that configuration, with N3's external address as the only editable field.";

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      {loading ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : !interfacesInfo ? (
        <EmptyState
          primaryText="No interface information available."
          secondaryText="Ella Core could not retrieve interface information from the API."
          extraContent={
            <Typography variant="body1" color="text.secondary">
              {description}
            </Typography>
          }
          button
          buttonText="Retry"
          onCreate={refetch}
        />
      ) : (
        <>
          <Box sx={{ mb: 2 }}>
            <Typography variant="h5" sx={{ mb: 0.5 }}>
              Network Interfaces
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {description}
            </Typography>
          </Box>

          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
              gap: 2,
              mt: 1,
            }}
          >
            {/* N2 */}
            <Box
              sx={{
                border: 1,
                borderColor: "divider",
                borderRadius: 2,
                p: 2,
              }}
            >
              <Stack
                direction="row"
                spacing={1}
                alignItems="center"
                justifyContent="space-between"
                sx={{ mb: 1 }}
              >
                <Typography variant="subtitle1">N2 (NGAP)</Typography>
                <Chip label="Control Plane" size="small" />
              </Stack>
              <Typography variant="body2" color="text.secondary">
                Address: <strong>{interfacesInfo.n2?.address ?? "—"}</strong>
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Port: <strong>{interfacesInfo.n2?.port ?? "—"}</strong>
              </Typography>
            </Box>

            {/* N3 */}
            <Box
              sx={{
                border: 1,
                borderColor: "divider",
                borderRadius: 2,
                p: 2,
              }}
            >
              <Stack
                direction="row"
                spacing={1}
                alignItems="center"
                justifyContent="space-between"
                sx={{ mb: 1 }}
              >
                <Typography variant="subtitle1">N3 (GTP-U)</Typography>
                <Stack direction="row" spacing={0.5} alignItems="center">
                  <Chip label="User Plane" size="small" />
                  {canEdit && (
                    <Tooltip title="Edit external address">
                      <IconButton
                        size="small"
                        onClick={() => setEditN3Open(true)}
                        color="primary"
                      >
                        <EditIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  )}
                </Stack>
              </Stack>
              <Typography variant="body2" color="text.secondary">
                Interface name:{" "}
                <strong>{interfacesInfo.n3?.name ?? "—"}</strong>
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Address: <strong>{interfacesInfo.n3?.address ?? "—"}</strong>
              </Typography>
              <Typography variant="body2" color="text.secondary">
                External address:{" "}
                <strong>{interfacesInfo.n3?.external_address || "—"}</strong>
              </Typography>
              {interfacesInfo.n3?.vlan && (
                <Typography variant="body2" color="text.secondary">
                  VLAN:{" "}
                  <strong>
                    {interfacesInfo.n3.vlan.vlan_id ?? "—"}
                    {interfacesInfo.n3.vlan.master_interface
                      ? ` on ${interfacesInfo.n3.vlan.master_interface}`
                      : ""}
                  </strong>
                </Typography>
              )}
            </Box>

            {/* N6 */}
            <Box
              sx={{
                border: 1,
                borderColor: "divider",
                borderRadius: 2,
                p: 2,
              }}
            >
              <Stack
                direction="row"
                spacing={1}
                alignItems="center"
                justifyContent="space-between"
                sx={{ mb: 1 }}
              >
                <Typography variant="subtitle1">N6 (External)</Typography>
                <Chip label="External Network" size="small" />
              </Stack>
              <Typography variant="body2" color="text.secondary">
                Interface name:{" "}
                <strong>{interfacesInfo.n6?.name ?? "—"}</strong>
              </Typography>
              {interfacesInfo.n6?.vlan && (
                <Typography variant="body2" color="text.secondary">
                  VLAN:{" "}
                  <strong>
                    {interfacesInfo.n6.vlan.vlan_id ?? "—"}
                    {interfacesInfo.n6.vlan.master_interface
                      ? ` on ${interfacesInfo.n6.vlan.master_interface}`
                      : ""}
                  </strong>
                </Typography>
              )}
            </Box>

            {/* API */}
            <Box
              sx={{
                border: 1,
                borderColor: "divider",
                borderRadius: 2,
                p: 2,
              }}
            >
              <Stack
                direction="row"
                spacing={1}
                alignItems="center"
                justifyContent="space-between"
                sx={{ mb: 1 }}
              >
                <Typography variant="subtitle1">API</Typography>
                <Chip label="Management" size="small" />
              </Stack>
              <Typography variant="body2" color="text.secondary">
                Address: <strong>{interfacesInfo.api?.address ?? "—"}</strong>
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Port: <strong>{interfacesInfo.api?.port ?? "—"}</strong>
              </Typography>
            </Box>
          </Box>
        </>
      )}

      {isEditN3Open && (
        <EditInterfaceN3Modal
          open
          onClose={() => setEditN3Open(false)}
          onSuccess={() => {
            showSnackbar(
              "N3 external address updated successfully.",
              "success",
            );
            refetch();
          }}
          initialData={{
            externalAddress: interfacesInfo?.n3?.external_address ?? "",
          }}
        />
      )}
    </Box>
  );
}
