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
  getFlowAccountingInfo,
  updateFlowAccountingInfo,
  type FlowAccountingInfo,
} from "@/queries/flow_accounting";
import QueryState from "@/components/QueryState";
import { useNetworkingContext } from "./types";

export default function FlowAccountingTab() {
  const { accessToken, canEdit, showSnackbar } = useNetworkingContext();

  const flowAccountingQuery = useQuery<FlowAccountingInfo>({
    queryKey: ["flow-accounting"],
    queryFn: () => getFlowAccountingInfo(accessToken || ""),
    enabled: !!accessToken,
    refetchOnWindowFocus: true,
  });

  const { mutate: setEnabled, isPending: mutating } = useMutation<
    void,
    unknown,
    boolean
  >({
    mutationFn: (enabled: boolean) =>
      updateFlowAccountingInfo(accessToken || "", enabled),
    onSuccess: () => {
      showSnackbar("Flow accounting updated successfully.", "success");
      void flowAccountingQuery.refetch();
    },
    onError: (error: unknown) => {
      const message =
        error instanceof Error
          ? error.message
          : "An unexpected error occurred.";
      showSnackbar(`Failed to update flow accounting: ${message}`, "error");
    },
  });

  const description =
    "Flow accounting records per-flow network usage (source/destination IP and port, protocol, bytes, packets) for each subscriber session. Disabling flow accounting reduces processing overhead and stops collection of new flow data.";

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      <Box sx={{ mb: 2 }}>
        <Typography variant="h5" sx={{ mb: 0.5 }}>
          Flow Accounting
        </Typography>
        <Typography variant="body2" color="textSecondary">
          {description}
        </Typography>
      </Box>

      <QueryState
        query={flowAccountingQuery}
        resource="flow accounting settings"
      >
        {(flowAccountingInfo) => (
          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={2}
            sx={{ alignItems: "center" }}
          >
            <FormControlLabel
              control={
                <Switch
                  checked={flowAccountingInfo.enabled}
                  onChange={(_, checked) => setEnabled(checked)}
                  disabled={!canEdit || mutating}
                />
              }
              label={
                flowAccountingInfo.enabled
                  ? "Flow accounting is ON"
                  : "Flow accounting is OFF"
              }
            />
          </Stack>
        )}
      </QueryState>
    </Box>
  );
}
