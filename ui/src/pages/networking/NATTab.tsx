import {
  Box,
  Typography,
  CircularProgress,
  Stack,
  FormControlLabel,
  Switch,
  Alert,
} from "@mui/material";
import { useMutation, useQuery } from "@tanstack/react-query";
import { getNATInfo, updateNATInfo, type NatInfo } from "@/queries/nat";
import { getBGPSettings, type BGPSettings } from "@/queries/bgp";
import { useNetworkingContext } from "./types";

export default function NATTab() {
  const { accessToken, canEdit, showSnackbar } = useNetworkingContext();
  const {
    data: natInfo,
    isLoading: loading,
    refetch,
  } = useQuery<NatInfo>({
    queryKey: ["nat"],
    queryFn: () => getNATInfo(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
  });

  const { data: bgpSettings } = useQuery<BGPSettings>({
    queryKey: ["bgp-settings"],
    queryFn: () => getBGPSettings(accessToken || ""),
    enabled: !!accessToken,
  });

  const { mutate: setNATEnabled, isPending: mutating } = useMutation<
    void,
    unknown,
    boolean
  >({
    mutationFn: (enabled: boolean) => updateNATInfo(accessToken || "", enabled),
    onSuccess: () => {
      showSnackbar("NAT updated successfully.", "success");
      refetch();
    },
    onError: (error: unknown) => {
      showSnackbar(`Failed to update NAT: ${String(error)}`, "error");
    },
  });

  const description =
    "Network Address Translation (NAT) simplifies networking as it lets subscribers use private IP addresses without requiring an external router. It uses Ella Core's N6 IP as the source for outbound traffic. Enabling NAT adds processing overhead and some niche protocols won't work (e.g., FTP active mode).";

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      {loading ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : (
        <>
          <Box sx={{ mb: 2 }}>
            <Typography variant="h5" sx={{ mb: 0.5 }}>
              NAT
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {description}
            </Typography>
          </Box>

          {bgpSettings?.enabled && !natInfo?.enabled && (
            <Alert severity="info" sx={{ mb: 2 }}>
              Enabling NAT will stop advertising subscriber routes via BGP.
            </Alert>
          )}

          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={2}
            alignItems="center"
          >
            <FormControlLabel
              control={
                <Switch
                  checked={!!natInfo?.enabled}
                  onChange={(_, checked) => setNATEnabled(checked)}
                  disabled={!canEdit || mutating || loading}
                />
              }
              label={natInfo?.enabled ? "NAT is ON" : "NAT is OFF"}
            />
          </Stack>
        </>
      )}
    </Box>
  );
}
