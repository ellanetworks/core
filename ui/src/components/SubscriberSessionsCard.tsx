// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
import AccessChip from "@/components/AccessChip";

interface SubscriberSessionsCardProps {
  sessions: SessionInfo[];
  accessType?: string;
  loading?: boolean;
}

const SubscriberSessionsCard: React.FC<SubscriberSessionsCardProps> = ({
  sessions,
  accessType,
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

  // Network slices (S-NSSAI) are 5G-only; mark them not applicable on 4G.
  const is4G = accessType === "4G";

  const columns: GridColDef<SessionInfo>[] = useMemo(
    () => [
      {
        field: "id",
        headerName: "ID",
        width: 60,
      },
      {
        field: "radio_access_type",
        headerName: "Access",
        width: 90,
        renderCell: (params: GridRenderCellParams<SessionInfo>) =>
          params.value ? (
            <Box sx={{ display: "flex", alignItems: "center", height: "100%" }}>
              <AccessChip label={params.value} />
            </Box>
          ) : (
            "—"
          ),
      },
      {
        field: "ipv4_address",
        headerName: "IP Address",
        flex: 0.8,
        minWidth: 120,
        renderCell: (params: GridRenderCellParams<SessionInfo>) => {
          const ipv4Address = params.value;
          const ipv6 = params.row.ipv6_prefix;
          if (!ipv4Address && !ipv6) return "—";
          return (
            <Box
              sx={{
                display: "flex",
                flexDirection: "column",
                alignItems: "flex-start",
                height: "100%",
                gap: 0.5,
                justifyContent: "center",
              }}
            >
              {ipv4Address && (
                <Typography
                  variant="body2"
                  sx={{ fontFamily: "monospace", fontSize: "0.8rem" }}
                >
                  {ipv4Address}
                </Typography>
              )}
              {ipv6 && (
                <Typography
                  variant="body2"
                  sx={{ fontFamily: "monospace", fontSize: "0.8rem" }}
                >
                  {ipv6}/64
                </Typography>
              )}
            </Box>
          );
        },
      },
      {
        field: "data_network",
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
      ...(is4G
        ? []
        : [
            {
              field: "slice",
              headerName: "Slice",
              flex: 0.6,
              minWidth: 100,
              renderCell: (params: GridRenderCellParams<SessionInfo>) => {
                const slice = params.row.slice;
                if (!slice) return "—";
                return slice.sd
                  ? `SST ${slice.sst} / SD ${slice.sd}`
                  : `SST ${slice.sst}`;
              },
            } as GridColDef<SessionInfo>,
          ]),
      {
        field: "ambr_uplink",
        headerName: "Session Bitrate Uplink",
        description: "Per-session uplink bitrate cap (Session-AMBR / APN-AMBR)",
        flex: 1,
        minWidth: 170,
      },
      {
        field: "ambr_downlink",
        headerName: "Session Bitrate Downlink",
        description:
          "Per-session downlink bitrate cap (Session-AMBR / APN-AMBR)",
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
    [theme, is4G],
  );

  return (
    <Box sx={{ mt: 4 }}>
      <Typography variant="h6" sx={{ mb: 2 }}>
        Sessions
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
            getRowId={(row) => row.id}
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
