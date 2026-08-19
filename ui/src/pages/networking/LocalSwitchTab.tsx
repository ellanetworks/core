// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import {
  Box,
  Typography,
  Stack,
  FormControlLabel,
  Switch,
} from "@mui/material";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  getLocalSwitchInfo,
  updateLocalSwitchInfo,
  type LocalSwitchInfo,
} from "@/queries/local_switch";
import QueryState from "@/components/QueryState";
import { useNetworkingContext } from "./types";

export default function LocalSwitchTab() {
  const { accessToken, canEdit, showSnackbar } = useNetworkingContext();

  const localSwitchQuery = useQuery<LocalSwitchInfo>({
    queryKey: ["local-switch"],
    queryFn: () => getLocalSwitchInfo(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
  });

  const { mutate: setEnabled, isPending: mutating } = useMutation<
    void,
    unknown,
    boolean
  >({
    mutationFn: (enabled: boolean) =>
      updateLocalSwitchInfo(accessToken || "", enabled),
    onSuccess: () => {
      showSnackbar("Local switch updated successfully.", "success");
      void localSwitchQuery.refetch();
    },
    onError: (error: unknown) => {
      const message =
        error instanceof Error
          ? error.message
          : "An unexpected error occurred.";
      showSnackbar(`Failed to update local switch: ${message}`, "error");
    },
  });

  const description =
    "Local switching enables UE-to-UE traffic forwarding within the same UPF. When enabled, uplink traffic destined for another UE on this UPF is forwarded directly.";

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      <Box sx={{ mb: 2 }}>
        <Typography variant="h5" sx={{ mb: 0.5 }}>
          Local Switch
        </Typography>
        <Typography variant="body2" color="textSecondary">
          {description}
        </Typography>
      </Box>

      <QueryState query={localSwitchQuery} resource="local switch settings">
        {(localSwitchInfo) => (
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={2}
            sx={{ alignItems: "center" }}
          >
            <FormControlLabel
              control={
                <Switch
                  checked={localSwitchInfo.enabled}
                  onChange={(_, checked) => setEnabled(checked)}
                  disabled={!canEdit || mutating}
                />
              }
              label={
                localSwitchInfo.enabled
                  ? "Local switch is ON"
                  : "Local switch is OFF"
              }
            />
          </Stack>
        )}
      </QueryState>
    </Box>
  );
}
