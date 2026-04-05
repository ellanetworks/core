import React, { useEffect } from "react";
import {
  Box,
  Typography,
  Button,
  Alert,
  IconButton,
  Divider,
  CircularProgress,
  Stack,
  Tooltip,
} from "@mui/material";
import {
  ContentCopy as CopyIcon,
  Refresh as RefreshIcon,
  WarningAmberRounded as WarningAmberRoundedIcon,
} from "@mui/icons-material";
import { useQuery } from "@tanstack/react-query";
import { getRadioEvent, type RadioEventContent } from "@/queries/radio_events";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useNavigate, Link as RouterLink } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { GenericMessageView } from "@/components/EventMessageRender";

export interface LogRow {
  id: string;
  timestamp: string;
  protocol: string;
  radio: string;
  address: string;
  messageType: string;
  direction: string;
}

const MonoBlock: React.FC<{ children: React.ReactNode; sxProp?: object }> = ({
  children,
  sxProp,
}) => (
  <Box
    component="pre"
    sx={{
      m: 0,
      p: 1.25,
      borderRadius: 1,
      bgcolor: "background.default",
      fontFamily:
        "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
      fontSize: 13,
      lineHeight: 1.5,
      whiteSpace: "pre-wrap",
      wordBreak: "break-word",
      overflowWrap: "anywhere",
      overflowX: "hidden",
      maxWidth: "100%",
      border: (t) => `1px solid ${t.palette.divider}`,
      ...sxProp,
    }}
  >
    {children}
  </Box>
);

const MetaRow: React.FC<{
  label: string;
  value?: string | null;
  full?: boolean;
}> = ({ label, value }) => (
  <Box
    sx={{
      display: "grid",
      gridTemplateColumns: "180px 1fr",
      alignItems: "baseline",
      gap: 1,
    }}
  >
    <Typography variant="caption" sx={{ color: "text.secondary" }}>
      {label}
    </Typography>
    <Typography variant="subtitle2">{value ?? "—"}</Typography>
  </Box>
);

export default function EventDetails({
  open,
  log,
}: {
  open: boolean;
  log: LogRow | null;
}) {
  const { showSnackbar } = useSnackbar();
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (authReady && !accessToken) navigate("/login");
  }, [authReady, accessToken, navigate]);

  const {
    data: decodedData,
    isLoading: isRetrieving,
    isError: isRetrieveError,
    error: retrieveError,
    refetch: refetchRadioEvent,
    isFetching: isRadioEventFetching,
  } = useQuery<RadioEventContent>({
    queryKey: ["decoded-log", log?.id],
    queryFn: async () => getRadioEvent(accessToken!, log!.id),
    enabled: open && !!log?.id && !!accessToken,
    staleTime: 60_000,
    gcTime: 5 * 60_000,
  });

  if (!authReady || !accessToken) return null;

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    showSnackbar("Copied to clipboard.", "success");
  };

  const stringify = (v: unknown): string => {
    if (v == null) return "";
    if (typeof v === "string") return v;
    try {
      return JSON.stringify(v, null, 2);
    } catch {
      return String(v);
    }
  };

  const decoded = decodedData?.decoded;
  const raw = decodedData?.raw;
  const rawString =
    raw == null
      ? ""
      : typeof raw === "string"
        ? raw
        : stringify(Array.from(raw));

  const decodedContent = (() => {
    if (isRetrieving || isRadioEventFetching) {
      return (
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <CircularProgress size={18} />
          <Typography variant="body2">Decoding…</Typography>
        </Box>
      );
    }
    if (isRetrieveError) {
      return (
        <Alert
          severity="error"
          action={
            <Button
              size="small"
              startIcon={<RefreshIcon fontSize="small" />}
              onClick={() => refetchRadioEvent()}
            >
              Retry
            </Button>
          }
          sx={{ mt: 0.5 }}
        >
          {retrieveError instanceof Error
            ? retrieveError.message
            : "Failed to decode message."}
        </Alert>
      );
    }
    if (!decodedData)
      return <Typography variant="body2">No decoded content.</Typography>;

    const pretty = <GenericMessageView decoded={decoded} />;

    return pretty ? (
      <>
        {pretty}
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, mt: 0.75 }}>
          <WarningAmberRoundedIcon
            fontSize="small"
            sx={{ color: (t) => t.palette.warning.main }}
            aria-hidden
          />
          <Typography variant="caption" sx={{ color: "text.secondary" }}>
            message decoding support is partial and content may be incomplete
          </Typography>
        </Box>
      </>
    ) : (
      <MonoBlock>{stringify(decoded)}</MonoBlock>
    );
  })();

  return (
    <Box
      sx={{
        flex: 1,
        minHeight: 0,
        display: "flex",
        flexDirection: "column",
        px: 2,
        pb: 2,
      }}
    >
      {/* Metadata — fixed height */}
      <Stack spacing={1.25} sx={{ my: 1.25, flexShrink: 0 }}>
        <MetaRow label="Timestamp" value={log?.timestamp} />
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: "180px 1fr",
            alignItems: "baseline",
            gap: 1,
          }}
        >
          <Typography variant="caption" sx={{ color: "text.secondary" }}>
            Radio
          </Typography>
          <Box>
            {log?.radio ? (
              <Typography
                variant="subtitle2"
                component={RouterLink}
                to={`/radios/${encodeURIComponent(log.radio)}`}
                sx={{
                  color: "link",
                  textDecoration: "underline",
                  "&:hover": { textDecoration: "underline" },
                }}
              >
                {log.radio}
              </Typography>
            ) : (
              <Typography variant="subtitle2" component="span">
                {"\u2014"}
              </Typography>
            )}
            {log?.address && (
              <Typography
                variant="subtitle2"
                component="span"
                sx={{ ml: 0.5, color: "text.secondary", fontWeight: 400 }}
              >
                ({log.address})
              </Typography>
            )}
          </Box>
        </Box>
        <MetaRow label="Direction" value={log?.direction} />
      </Stack>

      <Divider sx={{ flexShrink: 0 }} />

      {/* Decoded content — fills remaining space, scrolls internally */}
      <Box
        sx={{
          flexShrink: 0,
          mt: 1.5,
          mb: 0.5,
          display: "flex",
          justifyContent: "space-between",
        }}
      >
        <Typography variant="subtitle2">Decoded</Typography>
        <Tooltip title="Copy decoded content">
          <span>
            <IconButton
              size="small"
              onClick={() => handleCopy(stringify(decoded))}
              aria-label="Copy decoded content"
              disabled={decoded == null}
            >
              <CopyIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      </Box>
      <Box sx={{ flex: 1, minHeight: 0, overflow: "auto" }}>
        {decodedContent}
      </Box>

      {/* Raw — single line, always visible at bottom */}
      <Divider sx={{ flexShrink: 0, mt: 1.5 }} />
      <Box
        sx={{
          flexShrink: 0,
          mt: 1,
          display: "flex",
          alignItems: "center",
          gap: 1,
        }}
      >
        <Typography variant="subtitle2" sx={{ flexShrink: 0 }}>
          Raw
        </Typography>
        <Typography
          variant="body2"
          sx={{
            flex: 1,
            minWidth: 0,
            fontFamily:
              "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
            fontSize: 13,
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
            color: "text.secondary",
          }}
        >
          {rawString || "—"}
        </Typography>
        <Tooltip title="Copy raw message">
          <span>
            <IconButton
              size="small"
              onClick={() => handleCopy(rawString)}
              aria-label="Copy raw message"
              disabled={!rawString}
              sx={{ flexShrink: 0 }}
            >
              <CopyIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      </Box>
    </Box>
  );
}
