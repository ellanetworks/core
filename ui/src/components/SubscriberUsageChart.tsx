// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
import { useTheme } from "@mui/material/styles";
import {
  getUsage,
  type DailySubscriberUsage,
  type UsageResult,
} from "@/queries/usage";
import QueryState from "@/components/QueryState";
import { useAuth } from "@/contexts/AuthContext";
import {
  type DataUnit,
  UNIT_FACTORS,
  chooseUnitFromMax,
  formatBytesAutoUnit,
} from "@/utils/formatters";
import {
  DAILY_RANGES,
  resolveTimeRangeFilter,
  type TimeRangeFilter,
} from "@/components/TimeRangePicker";

interface SubscriberUsageChartProps {
  imsi: string;
  embedded?: boolean;
}

const DEFAULT_RANGE: TimeRangeFilter = { relative: "7d" };

type UsagePerDayRow = {
  date: string;
  uplink_bytes: number;
  downlink_bytes: number;
  total_bytes: number;
};

const SubscriberUsageChart: React.FC<SubscriberUsageChartProps> = ({
  imsi,
  embedded = false,
}) => {
  const { accessToken, authReady } = useAuth();
  const theme = useTheme();
  const usageQuery = useQuery<UsageResult>({
    queryKey: ["subscriber-usage-chart", imsi, DEFAULT_RANGE],
    queryFn: () => {
      const { from = "", to = "" } = resolveTimeRangeFilter(DEFAULT_RANGE, {
        ranges: DAILY_RANGES,
      });

      return getUsage(accessToken || "", from, to, imsi, "day");
    },
    enabled: authReady && !!accessToken && !!imsi,
    refetchInterval: 30000,
    retry: false,
    placeholderData: (prev) => prev,
  });

  const usageData = usageQuery.data;

  const dailyRows: UsagePerDayRow[] = useMemo(() => {
    if (!usageData) return [];
    const items: UsagePerDayRow[] = [];
    for (const usage of usageData as DailySubscriberUsage[]) {
      items.push({
        date: usage.date,
        uplink_bytes: usage.uplink_bytes,
        downlink_bytes: usage.downlink_bytes,
        total_bytes: usage.total_bytes,
      });
    }
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

      <QueryState
        query={usageQuery}
        resource="usage data"
        isEmpty={() => !hasData}
        empty={
          <Typography
            variant="body2"
            color="textSecondary"
            sx={{ py: 4, textAlign: "center" }}
          >
            No usage data available for this subscriber.
          </Typography>
        }
        loading={
          <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
            <CircularProgress size={28} />
          </Box>
        }
      >
        {() => (
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
                <SouthIcon
                  sx={{ fontSize: 16, color: theme.palette.chart.downlink }}
                />
                <Typography variant="body2" color="textSecondary">
                  {formatBytesAutoUnit(totalDownlink)}
                </Typography>
              </Box>
              <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                <NorthIcon
                  sx={{ fontSize: 16, color: theme.palette.chart.uplink }}
                />
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
                  color: theme.palette.chart.downlink,
                },
                {
                  dataKey: "uplink",
                  label: `Uplink (${unit})`,
                  stack: "total",
                  color: theme.palette.chart.uplink,
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
      </QueryState>
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
