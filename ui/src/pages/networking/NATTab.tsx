// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import {
  Box,
  Typography,
  Stack,
  FormControlLabel,
  Switch,
  Alert,
} from "@mui/material";
import { useMutation, useQuery } from "@tanstack/react-query";
import { getNATInfo, updateNATInfo, type NatInfo } from "@/queries/nat";
import { getBGPSettings, type BGPSettings } from "@/queries/bgp";
import QueryState from "@/components/QueryState";
import { useNetworkingContext } from "./types";

export default function NATTab() {
  const { accessToken, canEdit, showSnackbar } = useNetworkingContext();

  const natQuery = useQuery<NatInfo>({
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
      void natQuery.refetch();
    },
    onError: (error: unknown) => {
      const message =
        error instanceof Error
          ? error.message
          : "An unexpected error occurred.";
      showSnackbar(`Failed to update NAT: ${message}`, "error");
    },
  });

  const description =
    "Network Address Translation (NAT) simplifies networking as it lets subscribers use private IP addresses without requiring an external router. It uses Ella Core's N6 IP as the source for outbound traffic. Enabling NAT adds processing overhead and some niche protocols won't work (e.g., FTP active mode).";

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      <Box sx={{ mb: 2 }}>
        <Typography variant="h5" sx={{ mb: 0.5 }}>
          NAT
        </Typography>
        <Typography variant="body2" color="textSecondary">
          {description}
        </Typography>
      </Box>

      <QueryState query={natQuery} resource="NAT settings">
        {(natInfo) => (
          <>
            {bgpSettings?.enabled && !natInfo.enabled && (
              <Alert severity="info" sx={{ mb: 2 }}>
                Enabling NAT will stop advertising subscriber routes via BGP.
              </Alert>
            )}

            <Stack
              direction={{ xs: "column", sm: "row" }}
              spacing={2}
              sx={{ alignItems: "center" }}
            >
              <FormControlLabel
                control={
                  <Switch
                    checked={natInfo.enabled}
                    onChange={(_, checked) => setNATEnabled(checked)}
                    disabled={!canEdit || mutating}
                  />
                }
                label={natInfo.enabled ? "NAT is ON" : "NAT is OFF"}
              />
            </Stack>
          </>
        )}
      </QueryState>
    </Box>
  );
}
