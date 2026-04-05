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
import { NGAPMessageView } from "@/components/NGAPMessageRender";

function formatHexDump(base64: string): string {
  const bin = atob(base64);
  const bytes = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);

  const lines: string[] = [];
  for (let off = 0; off < bytes.length; off += 16) {
    const chunk = bytes.slice(off, off + 16);
    const offset = off.toString(16).padStart(4, "0");
    const hexParts: string[] = [];
    const asciiParts: string[] = [];
    for (let i = 0; i < 16; i++) {
      if (i === 8) hexParts.push("");
      if (i < chunk.length) {
        hexParts.push(chunk[i].toString(16).padStart(2, "0"));
        asciiParts.push(
          chunk[i] >= 0x20 && chunk[i] <= 0x7e
            ? String.fromCharCode(chunk[i])
            : ".",
        );
      } else {
        hexParts.push("  ");
        asciiParts.push(" ");
      }
    }
    lines.push(`${offset}  ${hexParts.join(" ")}  ${asciiParts.join("")}`);
  }
  return lines.join("\n");
}

export interface LogRow {
  id: string;
  timestamp: string;
  protocol: string;
  radio: string;
  address: string;
  messageType: string;
  direction: string;
}

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
  const rawString = raw == null ? "" : typeof raw === "string" ? raw : "";
  const hexDump = rawString ? formatHexDump(rawString) : "";

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
    if (!decodedData || !decoded)
      return <Typography variant="body2">No decoded content.</Typography>;

    return (
      <>
        <NGAPMessageView decoded={decoded} />
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, mt: 0.75 }}>
          <WarningAmberRoundedIcon
            fontSize="small"
            sx={{ color: (t) => t.palette.warning.main }}
            aria-hidden
          />
          <Typography variant="caption" sx={{ color: "text.secondary" }}>
            NGAP decoding is partial — some Information Elements may appear as
            raw values
          </Typography>
        </Box>
      </>
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
      <Box sx={{ flexShrink: 0, overflow: "auto", maxHeight: "50%" }}>
        {decodedContent}
      </Box>

      {/* Raw — hex dump */}
      <Divider sx={{ flexShrink: 0, mt: 1.5 }} />
      <Box
        sx={{
          flexShrink: 0,
          mt: 1,
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <Typography variant="subtitle2">Raw</Typography>
        <Tooltip title="Copy raw hex dump">
          <span>
            <IconButton
              size="small"
              onClick={() => handleCopy(hexDump)}
              aria-label="Copy raw hex dump"
              disabled={!hexDump}
            >
              <CopyIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      </Box>
      {hexDump ? (
        <Box
          component="pre"
          sx={{
            fontFamily:
              "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
            fontSize: 12,
            lineHeight: 1.4,
            color: "text.secondary",
            overflow: "auto",
            flex: 1,
            minHeight: 0,
            m: 0,
            mt: 0.5,
          }}
        >
          {hexDump}
        </Box>
      ) : (
        <Typography variant="body2" sx={{ color: "text.secondary", mt: 0.5 }}>
          —
        </Typography>
      )}
    </Box>
  );
}
