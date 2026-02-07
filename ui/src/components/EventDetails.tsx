import React, { useEffect, useState } from "react";
import {
  Box,
  Typography,
  Button,
  Alert,
  Collapse,
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
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";
import { GenericMessageView } from "@/components/EventMessageRender";

export interface LogRow {
  id: string;
  timestamp: string;
  protocol: string;
  local_address: string;
  remote_address: string;
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
    <Typography variant="body2" sx={{ fontWeight: 500 }}>
      {value ?? "—"}
    </Typography>
  </Box>
);

export default function EventDetails({
  open,
  log,
}: {
  open: boolean;
  log: LogRow | null;
}) {
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });
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
    setAlert({ message: "Copied to clipboard." });
    setTimeout(() => setAlert({ message: "" }), 1500);
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

  const content = (() => {
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

    const { decoded, raw } = decodedData;
    const pretty = <GenericMessageView decoded={decoded} />;

    return (
      <>
        {pretty ? (
          <>
            <Box
              sx={{ display: "flex", justifyContent: "space-between", mb: 0.5 }}
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
            {pretty}
            <Box
              sx={{ display: "flex", alignItems: "center", gap: 1, mt: 0.75 }}
            >
              <WarningAmberRoundedIcon
                fontSize="small"
                sx={{ color: (t) => t.palette.warning.main }}
                aria-hidden
              />
              <Typography variant="caption" sx={{ color: "text.secondary" }}>
                message decoding support is partial and content may be
                incomplete
              </Typography>
            </Box>
            <Divider sx={{ my: 1.5 }} />
          </>
        ) : (
          <>
            <Box
              sx={{ display: "flex", justifyContent: "space-between", mb: 0.5 }}
            >
              <Typography variant="subtitle2">Decoded (raw JSON)</Typography>
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
            <MonoBlock>{stringify(decoded)}</MonoBlock>
            <Divider sx={{ my: 1.5 }} />
          </>
        )}

        <Box sx={{ display: "flex", justifyContent: "space-between", mb: 0.5 }}>
          <Typography variant="subtitle2">
            Raw (base64 encoded bytes)
          </Typography>
          <Tooltip title="Copy raw message">
            <span>
              <IconButton
                size="small"
                onClick={() =>
                  handleCopy(
                    typeof raw === "string"
                      ? raw
                      : stringify(Array.from(raw ?? [])),
                  )
                }
                aria-label="Copy raw message"
                disabled={!raw}
              >
                <CopyIcon fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
        </Box>
        <MonoBlock>
          {typeof raw === "string" ? raw : stringify(Array.from(raw ?? []))}
        </MonoBlock>
      </>
    );
  })();

  return (
    <>
      <Box sx={{ px: 2, pt: 1 }}>
        <Collapse in={!!alert.message}>
          <Alert
            onClose={() => setAlert({ message: "" })}
            sx={{ mb: 1.5 }}
            severity="info"
          >
            {alert.message}
          </Alert>
        </Collapse>
      </Box>

      <Box sx={{ flex: 1, overflow: "auto", px: 2, pb: 2 }}>
        <Stack spacing={1.25} sx={{ my: 1.25 }}>
          <MetaRow label="Timestamp" value={log?.timestamp} />
          <MetaRow label="Protocol" value={log?.protocol} />
          <MetaRow label="Local Address" value={log?.local_address} />
          <MetaRow label="Remote Address" value={log?.remote_address} />
          <MetaRow label="Direction" value={log?.direction} />
          <MetaRow label="Message Type" value={log?.messageType} full />
        </Stack>

        <Divider sx={{ my: 1.5 }} />
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 1 }}>
          <Typography variant="subtitle1" sx={{ flex: 1 }}>
            Message Content
          </Typography>
        </Box>

        {content}
      </Box>
    </>
  );
}
