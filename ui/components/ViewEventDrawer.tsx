import React, { useEffect, useMemo, useState } from "react";
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
  Tooltip,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import {
  Close as CloseIcon,
  ContentCopy as CopyIcon,
  Refresh as RefreshIcon,
} from "@mui/icons-material";
import { useQuery } from "@tanstack/react-query";
import { decodeNetworkLog } from "@/queries/network_logs";
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
  const theme = useTheme();
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
    isLoading: isDecoding,
    isError: isDecodeError,
    error: decodeError,
    refetch: refetchDecode,
    isFetching: isDecodeFetching,
  } = useQuery({
    queryKey: ["decoded-log", log?.id],
    queryFn: async () => {
      return await decodeNetworkLog(accessToken!, log!.id);
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

  const renderDecoded = () => {
    if (isDecoding || isDecodeFetching) {
      return (
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <CircularProgress size={18} />
          <Typography variant="body2">Decoding…</Typography>
        </Box>
      );
    }

    if (isDecodeError) {
      return (
        <Alert
          severity="error"
          action={
            <Button
              size="small"
              startIcon={<RefreshIcon fontSize="small" />}
              onClick={() => refetchDecode()}
            >
              Retry
            </Button>
          }
          sx={{ mt: 0.5 }}
        >
          {decodeError instanceof Error
            ? decodeError.message
            : "Failed to decode message."}
        </Alert>
      );
    }

    if (!decodedData) {
      return <Typography variant="body2">No decoded content.</Typography>;
    }

    const maybeString =
      typeof decodedData === "string"
        ? decodedData
        : JSON.stringify(decodedData, null, 2);

    return (
      <>
        <Box sx={{ display: "flex", justifyContent: "flex-end", mb: 0.5 }}>
          <Tooltip title="Copy decoded message">
            <IconButton
              size="small"
              onClick={() => handleCopy(maybeString)}
              aria-label="Copy decoded message"
            >
              <CopyIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        </Box>
        <MonoBlock>{maybeString}</MonoBlock>
      </>
    );
  };

  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      PaperProps={{
        sx: {
          width: { xs: "100%", sm: 520, md: 640 },
          boxShadow: theme.shadows[8],
        },
      }}
      aria-label="View network log drawer"
    >
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
            Decoded Message
          </Typography>
        </Box>

        {renderDecoded()}
      </Box>
    </Drawer>
  );
};

export default ViewEventDrawer;
