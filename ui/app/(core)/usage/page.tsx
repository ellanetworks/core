"use client";

import { useMemo, useState } from "react";
import {
  Box,
  Typography,
  CircularProgress,
  Alert,
  Collapse,
  TextField,
} from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import useMediaQuery from "@mui/material/useMediaQuery";
import {
  DataGrid,
  type GridColDef,
  type GridPaginationModel,
} from "@mui/x-data-grid";
import {
  getUsagePerSubscriber,
  type UsagePerSubscriberResult,
} from "@/queries/usage";
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

  const { data, isLoading } = useQuery<UsagePerSubscriberResult>({
    queryKey: ["usagePerSubscriber", accessToken, startDate, endDate],
    queryFn: async () => {
      return getUsagePerSubscriber(accessToken || "", startDate, endDate, "");
    },
    enabled: !!accessToken && !!startDate && !!endDate,
  });

  const rows: UsageRow[] = useMemo(() => {
    if (!data) return [];

    const items: UsageRow[] = [];

    // data is UsagePerSubscriberResult:
    // Array<Record<string, SubscriberUsage>>
    for (const entry of data) {
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

    // Sort by total_bytes descending
    items.sort((a, b) => b.total_bytes - a.total_bytes);
    return items;
  }, [data]);

  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: "#F5F5F5" } },
      }),
    [theme],
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
        valueGetter: (value, row) => row.downlink_bytes,
      },
      {
        field: "uplink_bytes",
        headerName: "Usage (uplink)",
        flex: 1,
        minWidth: 180,
        type: "number",
        valueGetter: (value, row) => row.uplink_bytes,
      },
      {
        field: "total_bytes",
        headerName: "Usage (total)",
        flex: 1,
        minWidth: 180,
        type: "number",
        valueGetter: (value, row) => row.total_bytes,
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

      {isLoading && !data ? (
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
            </Box>
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
