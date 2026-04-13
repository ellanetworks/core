import {
  Box,
  Typography,
  CircularProgress,
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
import { useNetworkingContext } from "./types";

export default function FlowAccountingTab() {
  const { accessToken, canEdit, showSnackbar } = useNetworkingContext();
  const {
    data: flowAccountingInfo,
    isLoading: loading,
    refetch,
  } = useQuery<FlowAccountingInfo>({
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
      refetch();
    },
    onError: (error: unknown) => {
      showSnackbar(
        `Failed to update flow accounting: ${String(error)}`,
        "error",
      );
    },
  });

  const description =
    "Flow accounting records per-flow network usage (source/destination IP and port, protocol, bytes, packets) for each subscriber session. Disabling flow accounting reduces processing overhead and stops collection of new flow data.";

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
              Flow Accounting
            </Typography>
            <Typography variant="body2" color="textSecondary">
              {description}
            </Typography>
          </Box>

          <Stack
            direction={{ xs: "column", sm: "row" }}
            spacing={2}
            sx={{ alignItems: "center" }}
          >
            <FormControlLabel
              control={
                <Switch
                  checked={!!flowAccountingInfo?.enabled}
                  onChange={(_, checked) => setEnabled(checked)}
                  disabled={!canEdit || mutating || loading}
                />
              }
              label={
                flowAccountingInfo?.enabled
                  ? "Flow accounting is ON"
                  : "Flow accounting is OFF"
              }
            />
          </Stack>
        </>
      )}
    </Box>
  );
}
