// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { useMemo } from "react";
import { Box, TextField, Typography } from "@mui/material";
import { useTheme } from "@mui/material/styles";
import { useQuery } from "@tanstack/react-query";
import { type GridColDef } from "@mui/x-data-grid";
import { BarChart } from "@mui/x-charts/BarChart";
import EntityGrid from "@/components/grid/EntityGrid";
import EmptyState from "@/components/EmptyState";
import QueryState from "@/components/QueryState";
import ListPageHeader from "@/components/ListPageHeader";
import TimeRangePicker, {
  DAILY_RANGES,
  timeRangeFilter,
  timeRangeParams,
} from "@/components/TimeRangePicker";
import { useTimeRangeSearchParams } from "@/hooks/useTimeRangeSearchParams";
import { useDebouncedState } from "@/hooks/useDebouncedState";
import {
  getRoamingUsage,
  formatPLMN,
  type RoamingUsage,
} from "@/queries/roaming";
import { useAuth } from "@/contexts/AuthContext";
import {
  UNIT_FACTORS,
  chooseUnitFromMax,
  formatBytesAutoUnit,
} from "@/utils/formatters";
import { useRoamingContext } from "./types";

const IMSI_RE = /^\d{6,15}$/;

const FORMER_PARTNER = "Former partner";

type TotalRow = {
  id: string;
  plmn: string;
  partner: string;
  uplink_bytes: number;
  downlink_bytes: number;
  total_bytes: number;
};

const bytesColumn = (
  field: keyof TotalRow & string,
  headerName: string,
): GridColDef<TotalRow> => ({
  field,
  headerName,
  flex: 1,
  minWidth: 120,
  type: "number",
  valueFormatter: (value: unknown) =>
    value == null ? "" : formatBytesAutoUnit(Number(value)),
});

const totalColumns: GridColDef<TotalRow>[] = [
  { field: "partner", headerName: "Partner", flex: 1.3, minWidth: 160 },
  { field: "plmn", headerName: "PLMN", flex: 0.8, minWidth: 100 },
  bytesColumn("downlink_bytes", "Downlink"),
  bytesColumn("uplink_bytes", "Uplink"),
  bytesColumn("total_bytes", "Total"),
];

export default function UsageTab() {
  const { accessToken } = useRoamingContext();
  const { role } = useAuth();
  const canFilterByIMSI = role === "Admin" || role === "Read Only";
  const theme = useTheme();

  const [timeRange, setTimeRange] = useTimeRangeSearchParams({
    defaultPreset: "7d",
  });
  const timeFilter = useMemo(() => timeRangeFilter(timeRange), [timeRange]);

  const [imsiInput, appliedImsi, setImsiInput] = useDebouncedState();
  const imsiError = !!appliedImsi && !IMSI_RE.test(appliedImsi);
  const imsi = imsiError || !canFilterByIMSI ? "" : appliedImsi;

  const usageQuery = useQuery<RoamingUsage[]>({
    queryKey: ["roaming-usage", timeFilter, imsi],
    queryFn: () => {
      const { start = "", end = "" } = timeRangeParams(timeFilter, {
        ranges: DAILY_RANGES,
      });
      return getRoamingUsage(accessToken || "", start, end, imsi);
    },
    enabled: !!accessToken && !imsiError,
    placeholderData: (prev) => prev,
  });

  const totals: TotalRow[] = useMemo(() => {
    const byPLMN = new Map<string, TotalRow>();
    for (const usage of usageQuery.data ?? []) {
      const plmn = formatPLMN(usage);
      const total = byPLMN.get(plmn) ?? {
        id: plmn,
        plmn,
        partner: FORMER_PARTNER,
        uplink_bytes: 0,
        downlink_bytes: 0,
        total_bytes: 0,
      };
      if (usage.partner_name) total.partner = usage.partner_name;
      total.uplink_bytes += usage.uplink_bytes;
      total.downlink_bytes += usage.downlink_bytes;
      total.total_bytes += usage.total_bytes;
      byPLMN.set(plmn, total);
    }
    return [...byPLMN.values()].sort((a, b) => b.total_bytes - a.total_bytes);
  }, [usageQuery.data]);

  const daily = useMemo(() => {
    const byDate = new Map<string, { uplink: number; downlink: number }>();
    for (const row of usageQuery.data ?? []) {
      const day = byDate.get(row.date) ?? { uplink: 0, downlink: 0 };
      day.uplink += row.uplink_bytes;
      day.downlink += row.downlink_bytes;
      byDate.set(row.date, day);
    }
    return [...byDate.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([date, v]) => ({ date, ...v }));
  }, [usageQuery.data]);

  const unit = useMemo(
    () =>
      chooseUnitFromMax(
        daily.reduce((max, d) => Math.max(max, d.uplink, d.downlink), 0),
      ),
    [daily],
  );

  const chartDataset = useMemo(
    () =>
      daily.map((d) => ({
        date: d.date,
        downlink: d.downlink / UNIT_FACTORS[unit],
        uplink: d.uplink / UNIT_FACTORS[unit],
      })),
    [daily, unit],
  );

  return (
    <Box sx={{ width: "100%", mt: 2 }}>
      <ListPageHeader
        title="Usage"
        description="Data used by roamers, per partner network."
        variant="section"
        filters={
          <>
            <TimeRangePicker
              value={timeRange}
              onChange={setTimeRange}
              ranges={DAILY_RANGES}
              allowAnyTime={false}
            />
            {canFilterByIMSI && (
              <TextField
                size="small"
                label="IMSI"
                placeholder="All roamers"
                value={imsiInput}
                onChange={(e) => setImsiInput(e.target.value.trim())}
                error={imsiError}
                helperText={
                  imsiError ? "IMSI must be 6 to 15 digits" : undefined
                }
                sx={{ minWidth: 220 }}
              />
            )}
          </>
        }
      />

      <QueryState
        query={usageQuery}
        resource="roaming usage"
        isEmpty={(data) => data.length === 0}
        empty={
          <EmptyState
            primaryText="No roaming usage"
            secondaryText={
              imsi
                ? `No usage for IMSI ${imsi} in this time range.`
                : "No roamer used data in this time range."
            }
          />
        }
      >
        {() => (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
            <Box>
              <Typography variant="h6" sx={{ mb: 2 }}>
                Daily roaming usage ({imsi || "all roamers"}) in {unit}
              </Typography>
              <BarChart
                dataset={chartDataset}
                xAxis={[
                  { scaleType: "band", dataKey: "date", label: "Day (UTC)" },
                ]}
                yAxis={[{ label: `Usage (${unit})` }]}
                series={[
                  {
                    dataKey: "downlink",
                    label: `Downlink (${unit})`,
                    color: theme.palette.chart.downlink,
                  },
                  {
                    dataKey: "uplink",
                    label: `Uplink (${unit})`,
                    color: theme.palette.chart.uplink,
                  },
                ]}
                height={300}
                slotProps={{
                  legend: {
                    direction: "horizontal",
                    position: { vertical: "bottom", horizontal: "center" },
                  },
                }}
              />
            </Box>
            <EntityGrid<TotalRow>
              rows={totals}
              columns={totalColumns}
              getRowId={(row) => row.id}
              aria-label="Usage per PLMN"
            />
          </Box>
        )}
      </QueryState>
    </Box>
  );
}
