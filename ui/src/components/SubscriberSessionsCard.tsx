import React, { useMemo } from "react";
import { Box, Chip, CircularProgress, Typography } from "@mui/material";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import { Link as RouterLink } from "react-router-dom";
import {
  DataGrid,
  type GridColDef,
  type GridRenderCellParams,
} from "@mui/x-data-grid";
import type { SessionInfo } from "@/queries/subscribers";

interface SubscriberSessionsCardProps {
  sessions: SessionInfo[];
  loading?: boolean;
}

const SubscriberSessionsCard: React.FC<SubscriberSessionsCardProps> = ({
  sessions,
  loading = false,
}) => {
  const theme = useTheme();

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: theme.palette.backgroundSubtle } },
      }),
    [theme],
  );

  const columns: GridColDef<SessionInfo>[] = useMemo(
    () => [
      {
        field: "pdu_session_id",
        headerName: "ID",
        width: 60,
      },
      {
        field: "ipAddress",
        headerName: "IP Address",
        flex: 0.8,
        minWidth: 120,
        renderCell: (params: GridRenderCellParams<SessionInfo>) =>
          params.value ? (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                height: "100%",
              }}
            >
              <Typography
                variant="body2"
                sx={{ fontFamily: "monospace", fontSize: "0.8rem" }}
              >
                {params.value}
              </Typography>
            </Box>
          ) : (
            "—"
          ),
      },
      {
        field: "dnn",
        headerName: "Data Network",
        flex: 0.8,
        minWidth: 120,
        renderCell: (params: GridRenderCellParams<SessionInfo>) =>
          params.value ? (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                width: "100%",
                height: "100%",
              }}
            >
              <RouterLink
                to={`/networking/data-networks/${encodeURIComponent(params.value)}`}
                style={{ textDecoration: "none" }}
                onClick={(e: React.MouseEvent) => e.stopPropagation()}
              >
                <Typography
                  variant="body2"
                  sx={{
                    color: theme.palette.link,
                    textDecoration: "underline",
                    "&:hover": { textDecoration: "underline" },
                  }}
                >
                  {params.value}
                </Typography>
              </RouterLink>
            </Box>
          ) : (
            "—"
          ),
      },
      {
        field: "sst",
        headerName: "Slice",
        flex: 0.6,
        minWidth: 100,
        renderCell: (params: GridRenderCellParams<SessionInfo>) => {
          const sst = params.row.sst;
          const sd = params.row.sd;
          if (sst === undefined && !sd) return "—";
          return sd ? `SST ${sst} / SD ${sd}` : `SST ${sst}`;
        },
      },
      {
        field: "session_ambr_uplink",
        headerName: "Session Bitrate Uplink",
        description: "Per-session uplink bitrate cap (Session AMBR)",
        flex: 1,
        minWidth: 170,
      },
      {
        field: "session_ambr_downlink",
        headerName: "Session Bitrate Downlink",
        description: "Per-session downlink bitrate cap (Session AMBR)",
        flex: 1,
        minWidth: 180,
      },
      {
        field: "status",
        headerName: "Status",
        width: 100,
        renderCell: (params: GridRenderCellParams<SessionInfo>) => {
          const isActive = params.value?.toLowerCase() === "active";
          return (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                height: "100%",
              }}
            >
              <Chip
                size="small"
                label={params.value || "—"}
                color={isActive ? "success" : "default"}
                variant="filled"
              />
            </Box>
          );
        },
      },
    ],
    [theme],
  );

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h6" sx={{ mb: 2 }}>
        PDU Sessions
      </Typography>
      {loading ? (
        <Box sx={{ display: "flex", justifyContent: "center", py: 3 }}>
          <CircularProgress size={24} />
        </Box>
      ) : (
        <ThemeProvider theme={gridTheme}>
          <DataGrid<SessionInfo>
            rows={sessions}
            columns={columns}
            getRowId={(row) => row.pdu_session_id}
            disableRowSelectionOnClick
            disableColumnMenu
            hideFooter={sessions.length <= 25}
            pageSizeOptions={[25, 50, 100]}
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
              },
              "& .MuiDataGrid-footerContainer": {
                borderTop: "1px solid",
                borderColor: "divider",
              },
            }}
          />
        </ThemeProvider>
      )}
    </Box>
  );
};

export default SubscriberSessionsCard;
