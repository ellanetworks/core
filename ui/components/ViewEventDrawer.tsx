import React, { useEffect, useState } from "react";
import {
  Box,
  Drawer,
  Typography,
  Button,
  Alert,
  Collapse,
  IconButton,
  Divider,
  CircularProgress,
  Toolbar,
  Tooltip,
} from "@mui/material";
import {
  Close as CloseIcon,
  ContentCopy as CopyIcon,
  Refresh as RefreshIcon,
  WarningAmberRounded as WarningAmberRoundedIcon,
} from "@mui/icons-material";
import { useQuery } from "@tanstack/react-query";
import { getNetworkLog, type NetworkLogContent } from "@/queries/network_logs";
import { useRouter } from "next/navigation";
import { useAuth } from "@/contexts/AuthContext";

export interface LogRow {
  id: string;
  timestamp: string;
  protocol: string;
  local_address: string;
  remote_address: string;
  messageType: string;
  direction: string;
}

interface ViewEventDrawerProps {
  open: boolean;
  onClose: () => void;
  log: LogRow | null;
}

const MonoBlock: React.FC<{ children: React.ReactNode }> = ({ children }) => (
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
      overflowX: "auto",
      border: (t) => `1px solid ${t.palette.divider}`,
    }}
  >
    {children}
  </Box>
);

const ViewEventDrawer: React.FC<ViewEventDrawerProps> = ({
  open,
  onClose,
  log,
}) => {
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });
  const router = useRouter();
  const { accessToken, authReady } = useAuth();

  useEffect(() => {
    if (authReady && !accessToken) {
      router.push("/login");
    }
  }, [authReady, accessToken, router]);

  const {
    data: decodedData,
    isLoading: isRetrieving,
    isError: isRetrieveError,
    error: retrieveError,
    refetch: refetchNetworkLog,
    isFetching: isNetworkLogFetching,
  } = useQuery<NetworkLogContent>({
    queryKey: ["decoded-log", log?.id],
    queryFn: async () => {
      return await getNetworkLog(accessToken!, log!.id);
    },
    enabled: open && !!log?.id && !!accessToken,
    staleTime: 60_000,
    gcTime: 5 * 60_000,
  });

  if (!authReady || !accessToken) {
    return null;
  }

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

  const renderMessageContent = () => {
    if (isRetrieving || isNetworkLogFetching) {
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
              onClick={() => refetchNetworkLog()}
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

    if (!decodedData) {
      return <Typography variant="body2">No decoded content.</Typography>;
    }

    const { decoded, raw } = decodedData;

    return (
      <>
        <Box sx={{ display: "flex", justifyContent: "space-between", mb: 0.5 }}>
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
        <MonoBlock>{stringify(decoded)}</MonoBlock>

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

        <Divider sx={{ my: 1.5 }} />

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
  };

  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      variant="persistent"
      PaperProps={{
        sx: {
          width: { xs: "100%", sm: 520, md: 640 },
          boxShadow: (t) => t.shadows[8],
          display: "flex",
          flexDirection: "column",
          height: "100vh",
        },
      }}
    >
      <Toolbar />
      <Box
        sx={{
          position: "sticky",
          top: 0,
          zIndex: 1,
          px: 2,
          py: 1.5,
          borderBottom: (t) => `1px solid ${t.palette.divider}`,
          display: "flex",
          alignItems: "center",
          gap: 1,
        }}
      >
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography
            variant="subtitle2"
            sx={{ color: "text.secondary", lineHeight: 1 }}
          >
            {log?.protocol ?? "Log"}
          </Typography>
        </Box>

        <Tooltip title="Close">
          <IconButton onClick={onClose} aria-label="Close">
            <CloseIcon />
          </IconButton>
        </Tooltip>
      </Box>

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

      <Box sx={{ px: 2, pb: 2 }}>
        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
            gap: 1.25,
            my: 1.25,
          }}
        >
          <Typography variant="body2">
            <strong>Timestamp:</strong> {log?.timestamp ?? "—"}
          </Typography>
          <Typography variant="body2">
            <strong>Protocol:</strong> {log?.protocol ?? "—"}
          </Typography>
          <Typography variant="body2">
            <strong>Local Address:</strong> {log?.local_address ?? "—"}
          </Typography>
          <Typography variant="body2">
            <strong>Remote Address:</strong> {log?.remote_address ?? "—"}
          </Typography>
          <Typography variant="body2">
            <strong>Direction:</strong> {log?.direction ?? "—"}
          </Typography>
          <Typography
            variant="body2"
            sx={{ gridColumn: { xs: "auto", sm: "1 / span 2" } }}
          >
            <strong>Message Type:</strong> {log?.messageType ?? "—"}
          </Typography>
        </Box>

        <Divider sx={{ my: 1.5 }} />

        <Box sx={{ display: "flex", alignItems: "center", gap: 1, mb: 1 }}>
          <Typography variant="subtitle1" sx={{ flex: 1 }}>
            Message Content
          </Typography>
        </Box>

        {renderMessageContent()}
      </Box>
    </Drawer>
  );
};

export default ViewEventDrawer;
