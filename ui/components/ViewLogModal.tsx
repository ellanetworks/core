import React, { useMemo, useState } from "react";
import {
  Box,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Typography,
  Button,
  Alert,
  Collapse,
  IconButton,
  useMediaQuery,
  Divider,
} from "@mui/material";
import { useTheme } from "@mui/material/styles";
import { Close as CloseIcon } from "@mui/icons-material";

export interface LogRow {
  id: string;
  timestamp: string;
  event_id: string;
  event: string;
  direction: string;
  details: string; // JSON or free text
}

interface ViewLogModalProps {
  open: boolean;
  onClose: () => void;
  log: LogRow | null;
}

function parseDetails(details: string): Record<string, unknown> | null {
  try {
    return JSON.parse(details);
  } catch {
    return null;
  }
}

const ViewLogModal: React.FC<ViewLogModalProps> = ({ open, onClose, log }) => {
  const theme = useTheme();
  const fullScreen = useMediaQuery(theme.breakpoints.down("sm"));
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });

  const parsedDetails = useMemo(
    () => (log?.details ? parseDetails(log.details) : null),
    [log?.details],
  );

  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullScreen={fullScreen}
      fullWidth
      maxWidth="md"
      aria-labelledby="view-log-modal-title"
      aria-describedby="view-log-modal-description"
    >
      <DialogTitle id="view-log-modal-title" sx={{ pr: 6 }}>
        {log?.event ?? "Log Details"}
        <IconButton
          onClick={onClose}
          aria-label="Close"
          sx={{ position: "absolute", right: 8, top: 8 }}
        >
          <CloseIcon />
        </IconButton>
      </DialogTitle>

      <DialogContent dividers id="view-log-modal-description">
        <Collapse in={!!alert.message}>
          <Alert
            onClose={() => setAlert({ message: "" })}
            sx={{ mb: 2 }}
            severity="info"
          >
            {alert.message}
          </Alert>
        </Collapse>

        <Box
          sx={{
            display: "grid",
            gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
            gap: 1.5,
            mb: 2,
          }}
        >
          <Typography>
            <strong>Timestamp:</strong> {log?.timestamp ?? "—"}
          </Typography>

          <Typography>
            <strong>ID:</strong> {log?.event_id ?? "—"}
          </Typography>
        </Box>

        <Divider sx={{ my: 1.5 }} />

        <Typography variant="h6" sx={{ mb: 1.5 }}>
          Details
        </Typography>

        {parsedDetails ? (
          <Box sx={{ display: "grid", rowGap: 1 }}>
            {Object.entries(parsedDetails).map(([key, value]) => (
              <Typography key={key}>
                <strong>{key}:</strong> {String(value)}
              </Typography>
            ))}
          </Box>
        ) : (
          <Typography sx={{ fontFamily: "monospace" }}>
            {log?.details || "—"}
          </Typography>
        )}
      </DialogContent>

      <DialogActions>
        <Button onClick={onClose} sx={{ mr: 1.5 }}>
          Close
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ViewLogModal;
