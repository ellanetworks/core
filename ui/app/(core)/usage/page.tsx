"use client";

import { useMemo, useState } from "react";
import {
  Box,
  Typography,
  CircularProgress,
  Alert,
  Collapse,
  TextField,
  MenuItem,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import {
  DataGrid,
  type GridColDef,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import { BarChart } from "@mui/x-charts/BarChart";
import { getUsage, type UsageResult } from "@/queries/usage";
import { useAuth } from "@/contexts/AuthContext";
import { useQuery } from "@tanstack/react-query";

const MAX_WIDTH = 1400;

type UsageRow = {
  id: string;
  subscriber: string;
  uplink_bytes: number;
  downlink_bytes: number;
  total_bytes: number;
};

type UsagePerDayRow = {
  date: string;
  uplink_bytes: number;
  downlink_bytes: number;
  total_bytes: number;
};

type DataUnit = "B" | "KB" | "MB" | "GB" | "TB";

const UNIT_FACTORS: Record<DataUnit, number> = {
  B: 1,
  KB: 1024,
  MB: 1024 ** 2,
  GB: 1024 ** 3,
  TB: 1024 ** 4,
};

const chooseUnitFromMax = (maxBytes: number): DataUnit => {
  if (maxBytes >= UNIT_FACTORS.TB) return "TB";
  if (maxBytes >= UNIT_FACTORS.GB) return "GB";
  if (maxBytes >= UNIT_FACTORS.MB) return "MB";
  if (maxBytes >= UNIT_FACTORS.KB) return "KB";
  return "B";
};

const formatBytesWithUnit = (bytes: number, unit: DataUnit): string => {
  if (!Number.isFinite(bytes)) return "";
  const factor = UNIT_FACTORS[unit];
  const value = bytes / factor;
  const decimals = value >= 100 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(decimals)} ${unit}`;
};

const formatBytesAutoUnit = (bytes: number): string => {
  if (!Number.isFinite(bytes)) return "";
  const unit = chooseUnitFromMax(Math.abs(bytes));
  return formatBytesWithUnit(bytes, unit);
};

const getDefaultDateRange = () => {
  const today = new Date();
  const sevenDaysAgo = new Date();
  sevenDaysAgo.setDate(today.getDate() - 6); // last 7 days including today

  const format = (d: Date) => d.toISOString().slice(0, 10); // YYYY-MM-DD

  return {
    startDate: format(sevenDaysAgo),
    endDate: format(today),
  };
};

const SubscriberUsage = () => {
  const { accessToken } = useAuth();
  const theme = useTheme();
  const isSmDown = useMediaQuery(theme.breakpoints.down("sm"));

  const [{ startDate, endDate }, setDateRange] = useState(getDefaultDateRange);

  const [paginationModel, setPaginationModel] = useState<GridPaginationModel>({
    page: 0,
    pageSize: 25,
  });

  const [selectedSubscriber, setSelectedSubscriber] = useState<string>("");

  const {
    data: usagePerSubscriberData,
    isLoading: isUsagePerSubscriberLoading,
  } = useQuery<UsageResult>({
    queryKey: [
      "usagePerSubscriber",
      accessToken,
      startDate,
      endDate,
      selectedSubscriber,
    ],
    queryFn: async () => {
      return getUsage(
        accessToken || "",
        startDate,
        endDate,
        selectedSubscriber,
        "subscriber",
      );
    },
    enabled: !!accessToken && !!startDate && !!endDate,
  });

  const rows: UsageRow[] = useMemo(() => {
    if (!usagePerSubscriberData) return [];

    const items: UsageRow[] = [];

    for (const entry of usagePerSubscriberData) {
      const subscriber = Object.keys(entry)[0];
      const usage = entry[subscriber];

      if (!subscriber || !usage) continue;

      items.push({
        id: subscriber,
        subscriber,
        uplink_bytes: usage.uplink_bytes,
        downlink_bytes: usage.downlink_bytes,
        total_bytes: usage.total_bytes,
      });
    }

    items.sort((a, b) => b.total_bytes - a.total_bytes);
    return items;
  }, [usagePerSubscriberData]);

  const subscriberOptions = useMemo(
    () => rows.map((r) => r.subscriber),
    [rows],
  );

  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: "#F5F5F5" } },
      }),
    [theme],
  );

  const { data: usagePerDayData, isLoading: isUsagePerDayLoading } =
    useQuery<UsageResult>({
      queryKey: [
        "usagePerDay",
        accessToken,
        startDate,
        endDate,
        selectedSubscriber,
      ],
      queryFn: async () => {
        return getUsage(
          accessToken || "",
          startDate,
          endDate,
          selectedSubscriber,
          "day",
        );
      },
      enabled: !!accessToken && !!startDate && !!endDate,
    });

  const dailyRows: UsagePerDayRow[] = useMemo(() => {
    if (!usagePerDayData) return [];

    const items: UsagePerDayRow[] = [];

    for (const entry of usagePerDayData) {
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
  }, [usagePerDayData]);

  const maxBytesAcrossData = useMemo(() => {
    let max = 0;

    for (const row of rows) {
      if (row.uplink_bytes > max) max = row.uplink_bytes;
      if (row.downlink_bytes > max) max = row.downlink_bytes;
      if (row.total_bytes > max) max = row.total_bytes;
    }

    for (const row of dailyRows) {
      if (row.uplink_bytes > max) max = row.uplink_bytes;
      if (row.downlink_bytes > max) max = row.downlink_bytes;
      if (row.total_bytes > max) max = row.total_bytes;
    }

    return max;
  }, [rows, dailyRows]);

  const unit: DataUnit = useMemo(
    () => chooseUnitFromMax(maxBytesAcrossData),
    [maxBytesAcrossData],
  );

  const chartDataset = useMemo(
    () =>
      dailyRows.map((row) => {
        const factor = UNIT_FACTORS[unit];
        return {
          date: row.date,
          downlink: row.downlink_bytes / factor,
          uplink: row.uplink_bytes / factor,
        };
      }),
    [dailyRows, unit],
  );

  const columns: GridColDef<UsageRow>[] = useMemo(
    () => [
      {
        field: "subscriber",
        headerName: "Subscriber",
        flex: 1,
        minWidth: 200,
      },
      {
        field: "downlink_bytes",
        headerName: "Usage (downlink)",
        flex: 1,
        minWidth: 180,
        type: "number",
        valueFormatter: (value: any) =>
          value == null ? "" : formatBytesAutoUnit(Number(value)),
      },
      {
        field: "uplink_bytes",
        headerName: "Usage (uplink)",
        flex: 1,
        minWidth: 180,
        type: "number",
        valueFormatter: (value: any) =>
          value == null ? "" : formatBytesAutoUnit(Number(value)),
      },
      {
        field: "total_bytes",
        headerName: "Usage (total)",
        flex: 1,
        minWidth: 180,
        type: "number",
        valueFormatter: (value: any) =>
          value == null ? "" : formatBytesAutoUnit(Number(value)),
      },
    ],
    [],
  );

  const descriptionText =
    "View data usage per subscriber over a selected period.";

  const handleStartChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setDateRange((prev) => ({ ...prev, startDate: value }));
  };

  const handleEndChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setDateRange((prev) => ({ ...prev, endDate: value }));
  };

  const isInitialLoading =
    (isUsagePerSubscriberLoading && !usagePerSubscriberData) ||
    (isUsagePerDayLoading && !usagePerDayData);

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        pt: 6,
        pb: 4,
      }}
    >
      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}>
        <Collapse in={!!alert.message}>
          <Alert
            severity="success"
            onClose={() => setAlert({ message: "" })}
            sx={{ mb: 2 }}
          >
            {alert.message}
          </Alert>
        </Collapse>
      </Box>

      {isInitialLoading ? (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 6 }}>
          <CircularProgress />
        </Box>
      ) : (
        <>
          <Box
            sx={{
              width: "100%",
              maxWidth: MAX_WIDTH,
              px: { xs: 2, sm: 4 },
              mb: 3,
              display: "flex",
              flexDirection: "column",
              gap: 2,
            }}
          >
            <Typography variant="h4">Subscriber Usage</Typography>

            <Typography variant="body1" color="text.secondary">
              {descriptionText}
            </Typography>

            <Box
              sx={{
                display: "flex",
                flexDirection: { xs: "column", sm: "row" },
                gap: 2,
                alignItems: { xs: "flex-start", sm: "center" },
              }}
            >
              <TextField
                label="Start date"
                type="date"
                value={startDate}
                onChange={handleStartChange}
                InputLabelProps={{ shrink: true }}
                size="small"
              />
              <TextField
                label="End date"
                type="date"
                value={endDate}
                onChange={handleEndChange}
                InputLabelProps={{ shrink: true }}
                size="small"
              />
              <TextField
                select
                label="Subscriber"
                value={selectedSubscriber}
                onChange={(e) => setSelectedSubscriber(e.target.value)}
                size="small"
                sx={{ minWidth: 200 }}
              >
                <MenuItem value="">All subscribers</MenuItem>
                {subscriberOptions.map((sub) => (
                  <MenuItem key={sub} value={sub}>
                    {sub}
                  </MenuItem>
                ))}
              </TextField>
            </Box>
          </Box>

          <Box
            sx={{
              width: "100%",
              maxWidth: MAX_WIDTH,
              px: { xs: 2, sm: 4 },
              mb: 4,
            }}
          >
            <Typography variant="h6" sx={{ mb: 2 }}>
              Daily data usage ({selectedSubscriber || "all subscribers"}) in{" "}
              {unit}
            </Typography>

            <BarChart
              dataset={chartDataset}
              xAxis={[
                {
                  scaleType: "band",
                  dataKey: "date",
                },
              ]}
              yAxis={[
                {
                  label: `Usage (${unit})`,
                },
              ]}
              series={[
                { dataKey: "downlink", label: `Downlink (${unit})` },
                { dataKey: "uplink", label: `Uplink (${unit})` },
              ]}
              height={300}
            />
          </Box>

          <Box
            sx={{ width: "100%", maxWidth: MAX_WIDTH, px: { xs: 2, sm: 4 } }}
          >
            <ThemeProvider theme={gridTheme}>
              <DataGrid<UsageRow>
                rows={rows}
                columns={columns}
                getRowId={(row) => row.id}
                paginationModel={paginationModel}
                onPaginationModelChange={setPaginationModel}
                pageSizeOptions={[10, 25, 50, 100]}
                columnVisibilityModel={{ subscriber: !isSmDown }}
                sx={{
                  width: "100%",
                  border: 1,
                  borderColor: "divider",
                  "& .MuiDataGrid-cell": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-columnHeaders": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                    backgroundColor: "#F5F5F5",
                  },
                  "& .MuiDataGrid-footerContainer": {
                    borderTop: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-columnHeaderTitle": { fontWeight: "bold" },
                }}
              />
            </ThemeProvider>
          </Box>
        </>
      )}
    </Box>
  );
};

export default SubscriberUsage;
