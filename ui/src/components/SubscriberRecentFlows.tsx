import React, { useMemo } from "react";
import {
  Box,
  Card,
  CardContent,
  Chip,
  Typography,
  CircularProgress,
  Tooltip,
  Button,
} from "@mui/material";
import NorthIcon from "@mui/icons-material/North";
import SouthIcon from "@mui/icons-material/South";
import { DataGrid, type GridColDef } from "@mui/x-data-grid";
import { useQuery } from "@tanstack/react-query";
import {
  listFlowReports,
  type FlowReport,
  type ListFlowReportsResponse,
} from "@/queries/flow_reports";
import { useAuth } from "@/contexts/AuthContext";
import { Link as RouterLink } from "react-router-dom";
import {
  formatBytesAutoUnit,
  formatProtocol,
  formatRelativeTime,
} from "@/utils/formatters";

interface SubscriberRecentFlowsProps {
  imsi: string;
}

const SubscriberRecentFlows: React.FC<SubscriberRecentFlowsProps> = ({
  imsi,
}) => {
  const { accessToken, authReady } = useAuth();

  const { data: flowData, isLoading } = useQuery<ListFlowReportsResponse>({
    queryKey: ["subscriber-recent-flows", imsi],
    queryFn: () =>
      listFlowReports(accessToken || "", 1, 10, { subscriber_id: imsi }),
    enabled: authReady && !!accessToken && !!imsi,
    refetchInterval: 10000,
    placeholderData: (prev) => prev,
  });

  const rows = flowData?.items ?? [];

  const columns: GridColDef<FlowReport>[] = useMemo(
    () => [
      {
        field: "end_time",
        headerName: "Time",
        width: 100,
        sortable: false,
        valueFormatter: (value: string) =>
          value ? formatRelativeTime(value) : "",
      },
      {
        field: "direction",
        headerName: "Direction",
        width: 90,
        sortable: false,
        renderCell: (params) => {
          const dir = params.value as string;
          if (!dir) return null;
          const Icon = dir === "uplink" ? NorthIcon : SouthIcon;
          const title = dir === "uplink" ? "Uplink" : "Downlink";
          const color = dir === "uplink" ? "#FF9800" : "#4254FB";
          return (
            <Tooltip title={title}>
              <Box
                sx={{
                  width: "100%",
                  height: "100%",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  lineHeight: 0,
                  "& svg": { display: "block" },
                }}
              >
                <Icon fontSize="small" sx={{ color }} aria-label={title} />
              </Box>
            </Tooltip>
          );
        },
      },
      {
        field: "source_ip",
        headerName: "Source",
        flex: 1,
        minWidth: 140,
        sortable: false,
        renderCell: (params) => {
          const row = params.row as FlowReport;
          const proto = row.protocol;
          if (proto === 6 || proto === 17) {
            return `${row.source_ip}:${row.source_port}`;
          }
          return row.source_ip;
        },
      },
      {
        field: "destination_ip",
        headerName: "Destination",
        flex: 1,
        minWidth: 140,
        sortable: false,
        renderCell: (params) => {
          const row = params.row as FlowReport;
          const proto = row.protocol;
          if (proto === 6 || proto === 17) {
            return `${row.destination_ip}:${row.destination_port}`;
          }
          return row.destination_ip;
        },
      },
      {
        field: "protocol",
        headerName: "Protocol",
        width: 100,
        sortable: false,
        renderCell: (params) => {
          const value = params.value as number;
          if (value == null) return null;
          return (
            <Chip
              label={formatProtocol(value)}
              size="small"
              sx={{
                fontWeight: 600,
                fontSize: "0.75rem",
                height: 22,
              }}
            />
          );
        },
      },
      {
        field: "bytes",
        headerName: "Bytes",
        type: "number",
        width: 100,
        sortable: false,
        valueFormatter: (value: number) =>
          value == null ? "" : formatBytesAutoUnit(value),
      },
    ],
    [],
  );

  return (
    <Card variant="outlined">
      <CardContent>
        <Typography variant="h6" gutterBottom>
          Recent Flows
        </Typography>

        {isLoading ? (
          <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
            <CircularProgress size={28} />
          </Box>
        ) : rows.length === 0 ? (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ py: 4, textAlign: "center" }}
          >
            No flow reports available for this subscriber.
          </Typography>
        ) : (
          <>
            <DataGrid<FlowReport>
              rows={rows}
              columns={columns}
              getRowId={(row) => row.id}
              hideFooter
              disableColumnMenu
              density="compact"
              sx={{
                border: 0,
                "& .MuiDataGrid-columnHeaders": {
                  backgroundColor: "#F5F5F5",
                },
              }}
            />
            <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 1 }}>
              <Button
                component={RouterLink}
                to={`/traffic/flows?subscriber_id=${imsi}`}
                size="small"
              >
                View all flows →
              </Button>
            </Box>
          </>
        )}
      </CardContent>
    </Card>
  );
};

export default SubscriberRecentFlows;
