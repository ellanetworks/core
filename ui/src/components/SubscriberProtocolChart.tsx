import React, { useMemo } from "react";
import { Box, CircularProgress, Typography } from "@mui/material";
import { PieChart } from "@mui/x-charts/PieChart";
import { useQuery } from "@tanstack/react-query";
import {
  getFlowReportStats,
  type FlowReportStatsResponse,
} from "@/queries/flow_reports";
import { useAuth } from "@/contexts/AuthContext";
import {
  formatProtocol,
  PROTOCOL_CHIP_COLORS,
  PIE_COLORS,
} from "@/utils/formatters";

interface SubscriberProtocolChartProps {
  imsi: string;
}

const SubscriberProtocolChart: React.FC<SubscriberProtocolChartProps> = ({
  imsi,
}) => {
  const { accessToken, authReady } = useAuth();

  const { data: statsData, isLoading } = useQuery<FlowReportStatsResponse>({
    queryKey: ["subscriber-protocol-stats", imsi],
    queryFn: () =>
      // Do not force 'allow' here so the chart shows both allowed and dropped
      // flows by default. The API will return combined stats when action is
      // omitted.
      getFlowReportStats(accessToken || "", {
        subscriber_id: imsi,
      }),
    enabled: authReady && !!accessToken && !!imsi,
    refetchInterval: 10000,
    placeholderData: (prev) => prev,
  });

  const pieData = useMemo(() => {
    if (!statsData?.protocols?.length) return [];
    return statsData.protocols.map((p, i) => ({
      id: p.protocol,
      value: p.count,
      label: formatProtocol(p.protocol),
      color:
        PROTOCOL_CHIP_COLORS[p.protocol] ?? PIE_COLORS[i % PIE_COLORS.length],
    }));
  }, [statsData]);

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
        <CircularProgress size={28} />
      </Box>
    );
  }

  if (pieData.length === 0) {
    return (
      <Typography
        variant="body2"
        color="textSecondary"
        sx={{ py: 4, textAlign: "center" }}
      >
        No protocol data available.
      </Typography>
    );
  }

  return (
    <Box>
      <Typography
        variant="subtitle2"
        sx={{ mb: 1, color: "text.secondary", textAlign: "center" }}
      >
        Protocols
      </Typography>
      <PieChart
        series={[
          {
            data: pieData,
            innerRadius: 40,
            outerRadius: 90,
            paddingAngle: 2,
            cornerRadius: 3,
            valueFormatter: (item) => {
              const total = pieData.reduce((s, d) => s + d.value, 0);
              return total > 0
                ? `${((item.value / total) * 100).toFixed(1)}%`
                : "0%";
            },
          },
        ]}
        height={220}
        slotProps={{
          legend: {
            direction: "horizontal",
            position: {
              vertical: "bottom",
              horizontal: "center",
            },
          },
        }}
      />
    </Box>
  );
};

export default SubscriberProtocolChart;
