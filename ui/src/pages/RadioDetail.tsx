import React, { useEffect, useMemo, useState } from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Skeleton,
  Typography,
} from "@mui/material";
import {
  Link as RouterLink,
  useNavigate,
  useParams,
  Link,
} from "react-router-dom";
import { useTheme, createTheme, ThemeProvider } from "@mui/material/styles";
import {
  DataGrid,
  type GridColDef,
  type GridPaginationModel,
  type GridRenderCellParams,
} from "@mui/x-data-grid";
import { useQuery } from "@tanstack/react-query";
import {
  getRadio,
  type APIRadioDetail,
  type SupportedTAI,
  type Snssai,
} from "@/queries/radios";
import {
  listSubscribersByRadio,
  type APISubscriberSummary,
} from "@/queries/subscribers";
import { listRadioEvents, type APIRadioEvent } from "@/queries/radio_events";
import { useAuth } from "@/contexts/AuthContext";
import { formatRelativeTime } from "@/utils/formatters";
import { MAX_WIDTH, PAGE_PADDING_X } from "@/utils/layout";

const InfoRow: React.FC<{
  label: string;
  value?: React.ReactNode;
}> = ({ label, value }) => {
  const isEmpty = value === undefined || value === "" || value === null;
  const display = isEmpty ? "—" : value;

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "center",
        py: 0.75,
        minHeight: 40,
        "&:not(:last-child)": {
          borderBottom: "1px solid",
          borderColor: "divider",
        },
      }}
    >
      <Typography
        variant="body2"
        sx={{ color: "text.secondary", minWidth: 180, flexShrink: 0, mr: 2 }}
      >
        {label}
      </Typography>
      {typeof display === "string" || typeof display === "number" ? (
        <Typography variant="body2">{display}</Typography>
      ) : (
        display
      )}
    </Box>
  );
};

const normalizeRfc3339Offset = (s: string) =>
  s.replace(/([+-]\d{2})(\d{2})$/, "$1:$2");

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
    queryKey: ["radio-events", radio?.address],
    queryFn: () =>
      listRadioEvents(accessToken!, 1, 50, {
        remote_address: radio!.address,
      }),
    enabled: authReady && !!accessToken && !!radio?.address,
    refetchInterval: 5000,
  });

  const subscriberColumns: GridColDef<APISubscriberSummary>[] = useMemo(
    () => [
      {
        field: "imsi",
        headerName: "IMSI",
        flex: 1,
        minWidth: 200,
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              width: "100%",
              height: "100%",
            }}
          >
            <Link
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
            </Link>
          </Box>
        ),
      },
      {
        field: "registration",
        headerName: "Registration",
        width: 140,
        valueGetter: (_v, row) => Boolean(row?.status?.registered),
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => {
          const registered = Boolean(params.row?.status?.registered);
          return (
            <Chip
              size="small"
              label={registered ? "Registered" : "Deregistered"}
              color={registered ? "success" : "default"}
              variant="filled"
            />
          );
        },
      },
      {
        field: "ipAddress",
        headerName: "IP Address",
        width: 160,
        valueGetter: (_v, row: APISubscriberSummary) =>
          row?.status?.ipAddress ?? "",
        renderCell: (params: GridRenderCellParams<APISubscriberSummary>) => {
          const ip = params.row?.status?.ipAddress ?? "";
          return (
            <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
              {ip || "—"}
            </Typography>
          );
        },
      },
    ],
    [theme],
  );

  const eventRows = useMemo(() => {
    const items = eventsData?.items ?? [];
    return items.map((r) => ({
      ...r,
      timestamp_dt: r.timestamp
        ? new Date(normalizeRfc3339Offset(r.timestamp))
        : null,
    }));
  }, [eventsData?.items]);

  const eventColumns: GridColDef<APIRadioEvent>[] = useMemo(
    () => [
      {
        field: "timestamp_dt",
        headerName: "Timestamp",
        flex: 1,
        minWidth: 180,
        sortable: false,
        renderCell: (p) => (p.value ? p.value.toLocaleString() : ""),
      },
      {
        field: "message_type",
        headerName: "Message Type",
        flex: 1,
        minWidth: 220,
        sortable: false,
      },
      {
        field: "direction",
        headerName: "Direction",
        width: 120,
        sortable: false,
      },
    ],
    [],
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
        <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: PAGE_PADDING_X }}>
          <Skeleton variant="text" width={320} height={48} sx={{ mb: 3 }} />
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
              gap: 3,
            }}
          >
            <Skeleton variant="rounded" height={280} />
            <Skeleton variant="rounded" height={280} />
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
      <Box sx={{ width: "100%", maxWidth: MAX_WIDTH, px: PAGE_PADDING_X }}>
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

        {/* Two-column info cards */}
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", md: "1fr 1fr" },
            gap: 3,
          }}
        >
          {/* Radio Info Card */}
          <Card variant="outlined">
            <CardContent>
              <Typography variant="h6" sx={{ mb: 1 }}>
                Radio Info
              </Typography>
              <InfoRow label="Radio Name" value={radio.name} />
              <InfoRow label="Radio ID" value={radio.id} />
              <InfoRow label="Address" value={radio.address} />
              <InfoRow label="RAN Node Type" value={radio.ran_node_type} />
            </CardContent>
          </Card>

          {/* Connection Card */}
          <Card variant="outlined">
            <CardContent>
              <Typography variant="h6" sx={{ mb: 1 }}>
                Connection
              </Typography>
              <InfoRow
                label="Connected At"
                value={
                  radio.connected_at
                    ? `${new Date(radio.connected_at).toLocaleString()} (${formatRelativeTime(radio.connected_at)})`
                    : undefined
                }
              />
              <InfoRow
                label="Last Seen At"
                value={
                  radio.last_seen_at
                    ? `${new Date(radio.last_seen_at).toLocaleString()} (${formatRelativeTime(radio.last_seen_at)})`
                    : undefined
                }
              />
              <InfoRow
                label="Connected Subscribers"
                value={subscriberRowCount}
              />
            </CardContent>
          </Card>
        </Box>

        {/* Supported TAIs Card */}
        {radio.supported_tais && radio.supported_tais.length > 0 && (
          <Card variant="outlined" sx={{ mt: 3 }}>
            <CardContent>
              <Typography variant="h6" sx={{ mb: 2 }}>
                Supported TAIs
              </Typography>
              <ThemeProvider theme={gridTheme}>
                <DataGrid<SupportedTAI>
                  rows={radio.supported_tais.map((tai, idx) => ({
                    ...tai,
                    _idx: idx,
                  }))}
                  columns={[
                    {
                      field: "plmn",
                      headerName: "PLMN ID",
                      flex: 1,
                      minWidth: 140,
                      valueGetter: (_v, row: SupportedTAI) =>
                        `${row.tai.plmnID.mcc}-${row.tai.plmnID.mnc}`,
                    },
                    {
                      field: "tac",
                      headerName: "TAC",
                      flex: 0.6,
                      minWidth: 100,
                      valueGetter: (_v, row: SupportedTAI) => row.tai.tac,
                    },
                    {
                      field: "snssais",
                      headerName: "S-NSSAIs",
                      flex: 1.5,
                      minWidth: 200,
                      renderCell: (
                        params: GridRenderCellParams<SupportedTAI>,
                      ) => {
                        const snssais = params.row.snssais ?? [];
                        return (
                          <Box
                            sx={{
                              display: "flex",
                              gap: 0.5,
                              flexWrap: "wrap",
                              alignItems: "center",
                              height: "100%",
                            }}
                          >
                            {snssais.map((s: Snssai, i: number) => (
                              <Chip
                                key={i}
                                size="small"
                                label={
                                  s.sd
                                    ? `SST: ${s.sst} / SD: ${s.sd}`
                                    : `SST: ${s.sst}`
                                }
                              />
                            ))}
                          </Box>
                        );
                      },
                    },
                  ]}
                  getRowId={(row) =>
                    (row as SupportedTAI & { _idx: number })._idx
                  }
                  disableColumnMenu
                  disableRowSelectionOnClick
                  hideFooter
                  autoHeight
                  sx={{
                    border: 1,
                    borderColor: "divider",
                    "& .MuiDataGrid-cell": {
                      borderBottom: "1px solid",
                      borderColor: "divider",
                    },
                  }}
                />
              </ThemeProvider>
            </CardContent>
          </Card>
        )}

        {/* Connected Subscribers Card */}
        <Card variant="outlined" sx={{ mt: 3 }}>
          <CardContent>
            <Typography variant="h6" sx={{ mb: 2 }}>
              Connected Subscribers ({subscriberRowCount})
            </Typography>
            {subscriberRowCount === 0 ? (
              <Typography variant="body2" color="text.secondary">
                No subscribers are currently connected to this radio.
              </Typography>
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
                  pageSizeOptions={[10, 25, 50]}
                  disableColumnMenu
                  disableRowSelectionOnClick
                  autoHeight
                  sx={{
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
          </CardContent>
        </Card>

        {/* Recent Network Events Card */}
        <Card variant="outlined" sx={{ mt: 3 }}>
          <CardContent>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                mb: 2,
              }}
            >
              <Typography variant="h6">Recent Network Events</Typography>
              <Button
                component={RouterLink}
                to="/radios?tab=events"
                size="small"
                sx={{
                  color: theme.palette.link,
                  textDecoration: "underline",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                View all events →
              </Button>
            </Box>
            {eventRows.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                No recent events for this radio.
              </Typography>
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
                  sx={{
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
          </CardContent>
        </Card>
      </Box>
    </Box>
  );
};

export default RadioDetail;
