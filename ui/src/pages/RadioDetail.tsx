import React, { useEffect, useMemo, useState } from "react";
import {
  Box,
  Button,
  Chip,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@mui/material";
import { Link as RouterLink, useNavigate, useParams } from "react-router-dom";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  DataGrid,
  type GridColDef,
  type GridPaginationModel,
  type GridRenderCellParams,
} from "@mui/x-data-grid";
import { useQuery } from "@tanstack/react-query";
import { getRadio, type APIRadioDetail, type Snssai } from "@/queries/radios";
import EastIcon from "@mui/icons-material/East";
import WestIcon from "@mui/icons-material/West";
import {
  listSubscribersByRadio,
  type APISubscriberSummary,
} from "@/queries/subscribers";
import { listRadioEvents, type APIRadioEvent } from "@/queries/radio_events";
import { useAuth } from "@/contexts/AuthContext";
import { formatDateTime } from "@/utils/formatters";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const tableContainerSx = {
  border: 1,
  borderColor: "divider",
  borderRadius: 1,
} as const;

const labelCellSx = { fontWeight: 600, width: "35%" } as const;
const valueCellSx = { width: "65%" } as const;

// 10 compact DataGrid rows (33px) + header (36px) + pagination footer (52px)
const PANEL_HEIGHT = 421;

const ranNodeTypeChip = (t: string) => {
  const color =
    t === "gNB"
      ? "primary"
      : t === "ng-eNB"
        ? "secondary"
        : t === "N3IWF"
          ? "warning"
          : "default";
  return <Chip size="small" label={t} color={color} variant="outlined" />;
};

const RadioDetail: React.FC = () => {
  const { name } = useParams<{ name: string }>();
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();
  const theme = useTheme();

  const gridTheme = useMemo(
    () =>
      createTheme(theme, {
        palette: { DataGrid: { headerBg: theme.palette.backgroundSubtle } },
      }),
    [theme],
  );

  useEffect(() => {
    if (authReady && !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const {
    data: radio,
    isLoading,
    error,
  } = useQuery<APIRadioDetail>({
    queryKey: ["radio", name],
    queryFn: () => getRadio(accessToken!, name!),
    enabled: authReady && !!accessToken && !!name,
    refetchInterval: 5000,
  });

  const [subsPaginationModel, setSubsPaginationModel] =
    useState<GridPaginationModel>({ page: 0, pageSize: 10 });

  const subsPage = subsPaginationModel.page + 1;
  const subsPerPage = subsPaginationModel.pageSize;

  const { data: subscribersData } = useQuery({
    queryKey: ["subscribers-by-radio", name, subsPage, subsPerPage],
    queryFn: () =>
      listSubscribersByRadio(accessToken!, name!, subsPage, subsPerPage),
    enabled: authReady && !!accessToken && !!name,
    refetchInterval: 5000,
  });

  const { data: eventsData } = useQuery({
    queryKey: ["radio-events", name],
    queryFn: () =>
      listRadioEvents(accessToken!, 1, 12, {
        radio: name!,
      }),
    enabled: authReady && !!accessToken && !!name,
    refetchInterval: 5000,
  });

  const subscriberColumns: GridColDef<APISubscriberSummary>[] = useMemo(
    () => [
      {
        field: "imsi",
        headerName: "IMSI",
        flex: 1,
        minWidth: 140,
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <RouterLink
              to={`/subscribers/${params.row.imsi}`}
              style={{ textDecoration: "none" }}
              onClick={(e: React.MouseEvent) => e.stopPropagation()}
            >
              <Typography
                variant="body2"
                sx={{
                  fontFamily: "monospace",
                  color: theme.palette.link,
                  textDecoration: "underline",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                {params.row.imsi}
              </Typography>
            </RouterLink>
          </Box>
        ),
      },
      {
        field: "pduSessions",
        headerName: "PDU Sessions",
        flex: 0.5,
        minWidth: 100,
        valueGetter: (_v, row: APISubscriberSummary) =>
          row?.status?.num_pdu_sessions ?? 0,
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => {
          const count = params.row?.status?.num_pdu_sessions ?? 0;
          return (
            <Chip
              size="small"
              label={count}
              color={count > 0 ? "success" : "default"}
              variant="filled"
              sx={{ fontSize: "0.75rem" }}
            />
          );
        },
      },
      {
        field: "lastSeenAt",
        headerName: "Last Seen",
        flex: 0.8,
        minWidth: 120,
        valueGetter: (_v, row: APISubscriberSummary) =>
          row?.status?.lastSeenAt ?? "",
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => {
          const ts = params.row?.status?.lastSeenAt;
          return (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                width: "100%",
                height: "100%",
              }}
            >
              <Typography
                variant="body2"
                sx={ts ? {} : { color: "text.secondary" }}
              >
                {ts ? formatDateTime(ts) : "—"}
              </Typography>
            </Box>
          );
        },
      },
    ],
    [theme],
  );

  const eventRows = eventsData?.items ?? [];

  const eventColumns: GridColDef<APIRadioEvent>[] = useMemo(
    () => [
      {
        field: "timestamp",
        headerName: "Timestamp",
        flex: 1,
        minWidth: 140,
        sortable: false,
        renderCell: (p) => {
          const ts = p.row.timestamp;
          return (
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                width: "100%",
                height: "100%",
              }}
            >
              <Typography variant="body2">
                {ts ? formatDateTime(ts, { seconds: true }) : ""}
              </Typography>
            </Box>
          );
        },
      },
      {
        field: "message_type",
        headerName: "Message Type",
        flex: 1,
        minWidth: 160,
        sortable: false,
      },
      {
        field: "direction",
        headerName: "Direction",
        width: 110,
        sortable: false,
        renderCell: (p) => {
          const val = p.row.direction;
          if (!val) return null;
          const Icon = val === "outbound" ? EastIcon : WestIcon;
          const title =
            val === "inbound" ? "Receive (inbound)" : "Send (outbound)";
          const color =
            val === "inbound"
              ? theme.palette.success.main
              : theme.palette.info.main;
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
    ],
    [theme],
  );

  if (!authReady || isLoading) {
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
        <Box
          sx={{
            width: "100%",
            maxWidth: MAX_WIDTH,
            mx: "auto",
            px: PAGE_PADDING_X,
          }}
        >
          <Skeleton variant="text" width={320} height={48} sx={{ mb: 3 }} />
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
              gap: 3,
            }}
          >
            <Skeleton variant="rounded" height={320} />
            <Skeleton variant="rounded" height={320} />
          </Box>
          <Skeleton variant="rounded" height={200} sx={{ mt: 3 }} />
        </Box>
      </Box>
    );
  }

  if (error) {
    return (
      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
          mt: 6,
          gap: 2,
        }}
      >
        <Typography color="error">
          {error instanceof Error ? error.message : "Failed to load radio."}
        </Typography>
        <Button variant="outlined" component={RouterLink} to="/radios">
          Back to Radios
        </Button>
      </Box>
    );
  }

  if (!radio) return null;

  const subscriberRows = subscribersData?.items ?? [];
  const subscriberRowCount = subscribersData?.total_count ?? 0;
  const tais = radio.supported_tais ?? [];

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
      <Box
        sx={{
          width: "100%",
          maxWidth: MAX_WIDTH,
          mx: "auto",
          px: PAGE_PADDING_X,
        }}
      >
        {/* Header / Breadcrumb */}
        <Box sx={{ mb: 3 }}>
          <Typography
            variant="h4"
            sx={{ display: "flex", alignItems: "baseline", gap: 0 }}
          >
            <Typography
              component={RouterLink}
              to="/radios"
              variant="h4"
              sx={{
                color: "text.secondary",
                textDecoration: "none",
                "&:hover": { textDecoration: "underline" },
              }}
            >
              Radios
            </Typography>
            <Typography
              component="span"
              variant="h4"
              sx={{ color: "text.secondary", mx: 1 }}
            >
              /
            </Typography>
            <Typography component="span" variant="h4">
              {radio.name}
            </Typography>
          </Typography>
        </Box>

        {/* Two-column layout: Radio Info (left) + Connected Subscribers (right) */}
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            gap: 3,
            alignItems: "start",
          }}
        >
          {/* Radio Info Table */}
          <Box>
            <Typography variant="h6" sx={{ mb: 1 }}>
              Radio Info
            </Typography>
            <TableContainer
              sx={{
                ...tableContainerSx,
                height: PANEL_HEIGHT,
                overflow: "auto",
              }}
            >
              <Table size="small">
                <TableBody>
                  <TableRow>
                    <TableCell sx={labelCellSx}>Name</TableCell>
                    <TableCell sx={valueCellSx}>{radio.name}</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell sx={labelCellSx}>ID</TableCell>
                    <TableCell sx={valueCellSx}>{radio.id || "—"}</TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell sx={labelCellSx}>Address</TableCell>
                    <TableCell sx={valueCellSx}>
                      {radio.address || "—"}
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell sx={labelCellSx}>RAN Node Type</TableCell>
                    <TableCell sx={valueCellSx}>
                      {ranNodeTypeChip(radio.ran_node_type)}
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell sx={labelCellSx}>Connected At</TableCell>
                    <TableCell sx={valueCellSx}>
                      {radio.connected_at
                        ? formatDateTime(radio.connected_at)
                        : "—"}
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell sx={labelCellSx}>Last Seen At</TableCell>
                    <TableCell sx={valueCellSx}>
                      {radio.last_seen_at
                        ? formatDateTime(radio.last_seen_at)
                        : "—"}
                    </TableCell>
                  </TableRow>
                  {tais.length > 0 && (
                    <TableRow sx={{ "& td": { borderBottom: "none" } }}>
                      <TableCell sx={labelCellSx}>Supported TAIs</TableCell>
                      <TableCell sx={valueCellSx}>
                        <Table size="small" sx={{ m: -1 }}>
                          <TableHead>
                            <TableRow
                              sx={{
                                "& th": {
                                  py: 0.5,
                                  fontWeight: 600,
                                  fontSize: "0.75rem",
                                  color: "text.secondary",
                                },
                              }}
                            >
                              <TableCell sx={{ pl: 0, width: "30%" }}>
                                PLMN ID
                              </TableCell>
                              <TableCell sx={{ width: "20%" }}>TAC</TableCell>
                              <TableCell sx={{ pr: 0 }}>S-NSSAIs</TableCell>
                            </TableRow>
                          </TableHead>
                          <TableBody>
                            {tais.map((tai, idx) => (
                              <TableRow
                                key={idx}
                                sx={{
                                  "& td": {
                                    borderBottom:
                                      idx < tais.length - 1
                                        ? undefined
                                        : "none",
                                    py: 0.5,
                                  },
                                }}
                              >
                                <TableCell sx={{ pl: 0, width: "30%" }}>
                                  {tai.tai.plmnID.mcc}-{tai.tai.plmnID.mnc}
                                </TableCell>
                                <TableCell sx={{ width: "20%" }}>
                                  {tai.tai.tac}
                                </TableCell>
                                <TableCell sx={{ pr: 0 }}>
                                  <Box
                                    sx={{
                                      display: "flex",
                                      gap: 0.5,
                                      flexWrap: "wrap",
                                    }}
                                  >
                                    {(tai.snssais ?? []).map(
                                      (s: Snssai, i: number) => (
                                        <Chip
                                          key={i}
                                          size="small"
                                          label={
                                            s.sd
                                              ? `SST: ${s.sst} / SD: ${s.sd}`
                                              : `SST: ${s.sst}`
                                          }
                                        />
                                      ),
                                    )}
                                  </Box>
                                </TableCell>
                              </TableRow>
                            ))}
                          </TableBody>
                        </Table>
                      </TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>

          {/* Connected Subscribers */}
          <Box>
            <Typography variant="h6" sx={{ mb: 1 }}>
              Connected Subscribers ({subscriberRowCount})
            </Typography>
            {subscriberRowCount === 0 ? (
              <TableContainer
                sx={{ ...tableContainerSx, height: PANEL_HEIGHT }}
              >
                <Box sx={{ p: 3, textAlign: "center" }}>
                  <Typography variant="body2" color="text.secondary">
                    No subscribers are currently connected to this radio.
                  </Typography>
                </Box>
              </TableContainer>
            ) : (
              <ThemeProvider theme={gridTheme}>
                <DataGrid<APISubscriberSummary>
                  rows={subscriberRows}
                  columns={subscriberColumns}
                  getRowId={(row) => row.imsi}
                  paginationMode="server"
                  rowCount={subscriberRowCount}
                  paginationModel={subsPaginationModel}
                  onPaginationModelChange={setSubsPaginationModel}
                  pageSizeOptions={[10]}
                  disableColumnMenu
                  disableRowSelectionOnClick
                  density="compact"
                  sx={{
                    height: PANEL_HEIGHT,
                    border: 1,
                    borderColor: "divider",
                    "& .MuiDataGrid-cell": {
                      borderBottom: "1px solid",
                      borderColor: "divider",
                    },
                  }}
                />
              </ThemeProvider>
            )}
          </Box>
        </Box>

        {/* Recent Network Events */}
        <Box sx={{ mt: 3 }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              mb: 1,
            }}
          >
            <Typography variant="h6">Recent Network Events</Typography>
            <Button
              component={RouterLink}
              to={`/radios/events?radio=${encodeURIComponent(radio.name)}`}
              size="small"
              sx={{
                color: theme.palette.link,
                textDecoration: "underline",
                "&:hover": { textDecoration: "underline" },
              }}
            >
              View all events for {radio.name} →
            </Button>
          </Box>
          {eventRows.length === 0 ? (
            <TableContainer sx={tableContainerSx}>
              <Box sx={{ p: 3, textAlign: "center" }}>
                <Typography variant="body2" color="text.secondary">
                  No recent events for this radio.
                </Typography>
              </Box>
            </TableContainer>
          ) : (
            <ThemeProvider theme={gridTheme}>
              <DataGrid
                rows={eventRows}
                columns={eventColumns}
                getRowId={(row) => row.id}
                disableColumnMenu
                disableRowSelectionOnClick
                hideFooter
                autoHeight
                density="compact"
                onRowClick={(params) => {
                  navigate(
                    `/radios/events?radio=${encodeURIComponent(radio.name)}&event=${params.row.id}`,
                  );
                }}
                sx={{
                  border: 1,
                  borderColor: "divider",
                  "& .MuiDataGrid-cell": {
                    borderBottom: "1px solid",
                    borderColor: "divider",
                  },
                  "& .MuiDataGrid-row": { cursor: "pointer" },
                }}
              />
            </ThemeProvider>
          )}
        </Box>
      </Box>
    </Box>
  );
};

export default RadioDetail;
