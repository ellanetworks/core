import React, { useMemo } from "react";
import {
  Box,
  Card,
  CardContent,
  Typography,
  CircularProgress,
} from "@mui/material";
import NorthIcon from "@mui/icons-material/North";
import SouthIcon from "@mui/icons-material/South";
import { BarChart } from "@mui/x-charts/BarChart";
import { useQuery } from "@tanstack/react-query";
import { getUsage, type UsageResult } from "@/queries/usage";
import { useAuth } from "@/contexts/AuthContext";
import {
  type DataUnit,
  UNIT_FACTORS,
  chooseUnitFromMax,
  formatBytesAutoUnit,
  UPLINK_COLOR,
  DOWNLINK_COLOR,
} from "@/utils/formatters";

interface SubscriberUsageChartProps {
  imsi: string;
  /** When true, renders without Card wrapper (for embedding in a parent card). */
  embedded?: boolean;
}

type UsagePerDayRow = {
  date: string;
  uplink_bytes: number;
  downlink_bytes: number;
  total_bytes: number;
};

const getDateRange7Days = () => {
  const today = new Date();
  const sevenDaysAgo = new Date();
  sevenDaysAgo.setDate(today.getDate() - 6);
  const format = (d: Date) => d.toISOString().slice(0, 10);
  return { startDate: format(sevenDaysAgo), endDate: format(today) };
};

const SubscriberUsageChart: React.FC<SubscriberUsageChartProps> = ({
  imsi,
  embedded = false,
}) => {
  const { accessToken, authReady } = useAuth();
  // Date range is computed once on mount. If the page stays open past midnight
  // the chart won't shift to include the new day until the component remounts.
  const { startDate, endDate } = useMemo(() => getDateRange7Days(), []);

  const { data: usageData, isLoading } = useQuery<UsageResult>({
    queryKey: ["subscriber-usage-chart", imsi, startDate, endDate],
    queryFn: () => getUsage(accessToken || "", startDate, endDate, imsi, "day"),
    enabled: authReady && !!accessToken && !!imsi,
    refetchInterval: 30000,
    placeholderData: (prev) => prev,
  });

  const dailyRows: UsagePerDayRow[] = useMemo(() => {
    if (!usageData) return [];
    const items: UsagePerDayRow[] = [];
    for (const entry of usageData) {
      const date = Object.keys(entry)[0];
      const usage = entry[date];
      if (!date || !usage) continue;
      items.push({
        date,
        uplink_bytes: usage.uplink_bytes,
        downlink_bytes: usage.downlink_bytes,
        total_bytes: usage.total_bytes,
      });
    }
    items.sort((a, b) => a.date.localeCompare(b.date));
    return items;
  }, [usageData]);

  const maxBytes = useMemo(() => {
    let max = 0;
    for (const row of dailyRows) {
      const sum = row.uplink_bytes + row.downlink_bytes;
      if (sum > max) max = sum;
    }
    return max;
  }, [dailyRows]);

  const unit: DataUnit = useMemo(() => chooseUnitFromMax(maxBytes), [maxBytes]);

  const chartDataset = useMemo(
    () =>
      dailyRows.map((row) => {
        const factor = UNIT_FACTORS[unit];
        const d = new Date(row.date + "T00:00:00");
        const label = d.toLocaleDateString("en-US", {
          month: "short",
          day: "numeric",
        });
        return {
          date: label,
          downlink: row.downlink_bytes / factor,
          uplink: row.uplink_bytes / factor,
        };
      }),
    [dailyRows, unit],
  );

  const totalUplink = useMemo(
    () => dailyRows.reduce((sum, r) => sum + r.uplink_bytes, 0),
    [dailyRows],
  );
  const totalDownlink = useMemo(
    () => dailyRows.reduce((sum, r) => sum + r.downlink_bytes, 0),
    [dailyRows],
  );

  const hasData =
    dailyRows.length > 0 && (totalUplink > 0 || totalDownlink > 0);

  const content = (
    <>
      <Typography
        variant="subtitle2"
        sx={{ mb: 1, color: "text.secondary", textAlign: "center" }}
      >
        Usage (last 7 days)
      </Typography>

      {isLoading ? (
        <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
          <CircularProgress size={28} />
        </Box>
      ) : !hasData ? (
        <Typography
          variant="body2"
          color="textSecondary"
          sx={{ py: 4, textAlign: "center" }}
        >
          No usage data available for this subscriber.
        </Typography>
      ) : (
        <>
          <Box
            sx={{
              display: "flex",
              gap: 3,
              mb: 1,
              justifyContent: "center",
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
              <SouthIcon sx={{ fontSize: 16, color: DOWNLINK_COLOR }} />
              <Typography variant="body2" color="textSecondary">
                {formatBytesAutoUnit(totalDownlink)}
              </Typography>
            </Box>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
              <NorthIcon sx={{ fontSize: 16, color: UPLINK_COLOR }} />
              <Typography variant="body2" color="textSecondary">
                {formatBytesAutoUnit(totalUplink)}
              </Typography>
            </Box>
          </Box>
          <BarChart
            dataset={chartDataset}
            xAxis={[{ scaleType: "band", dataKey: "date" }]}
            yAxis={[{ label: `Usage (${unit})` }]}
            series={[
              {
                dataKey: "downlink",
                label: `Downlink (${unit})`,
                stack: "total",
                color: DOWNLINK_COLOR,
              },
              {
                dataKey: "uplink",
                label: `Uplink (${unit})`,
                stack: "total",
                color: UPLINK_COLOR,
              },
            ]}
            height={250}
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
        </>
      )}
    </>
  );

  if (embedded) return <Box>{content}</Box>;

  return (
    <Card variant="outlined">
      <CardContent>{content}</CardContent>
    </Card>
  );
};

export default SubscriberUsageChart;
