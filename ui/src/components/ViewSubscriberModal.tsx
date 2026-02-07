import React, { useState, useEffect } from "react";
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
} from "@mui/material";
import { ContentCopy as CopyIcon } from "@mui/icons-material";
import { getSubscriber } from "@/queries/subscribers";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/contexts/AuthContext";

interface ViewSubscriberModalProps {
  open: boolean;
  onClose: () => void;
  imsi: string;
}

const ViewSubscriberModal: React.FC<ViewSubscriberModalProps> = ({
  open,
  onClose,
  imsi,
}) => {
  const navigate = useNavigate();
  const { accessToken, authReady } = useAuth();
  if (!authReady || !accessToken) navigate("/login");

  const [subscriberData, setSubscriberData] = useState({
    imsi: "",
    key: "",
    opc: "",
    sequenceNumber: "",
    policyName: "",
  });
  const [, setLoading] = useState(false);
  const [alert, setAlert] = useState<{ message: string }>({ message: "" });
  const [keyObfuscated, setKeyObfuscated] = useState(true);
  const [opcObfuscated, setOPcObfuscated] = useState(true);

  useEffect(() => {
    const fetchSubscriberData = async () => {
      if (!imsi || !open) return;

      setLoading(true);
      setAlert({ message: "" });

      try {
        if (!accessToken) return;
        const data = await getSubscriber(accessToken, imsi);
        setSubscriberData({
          imsi: data.imsi,
          key: data.key,
          opc: data.opc,
          sequenceNumber: data.sequenceNumber,
          policyName: data.policyName,
        });
      } catch (error: unknown) {
        let errorMessage = "Unknown error occurred.";
        if (error instanceof Error) {
          errorMessage = error.message;
        }
        setAlert({
          message: `Failed to get subscriber: ${errorMessage}`,
        });
        console.error("Error fetching subscriber data:", error);
      } finally {
        setLoading(false);
      }
    };

    fetchSubscriberData();
  }, [imsi, open, accessToken]);

  const handleCopy = async (value: string, label: string) => {
    if (!navigator.clipboard) {
      console.error(`Clipboard API not available.`);
      setAlert({
        message: `Clipboard API not available. Please use HTTPS or try a different browser.`,
      });
      return;
    }

    try {
      await navigator.clipboard.writeText(value);
      setAlert({ message: `${label} copied to clipboard!` });
    } catch (error) {
      console.error(`Failed to copy ${label}:`, error);
      setAlert({ message: `Failed to copy ${label}.` });
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      aria-labelledby="view-subscriber-modal-title"
      aria-describedby="view-subscriber-modal-description"
    >
      <DialogTitle>Subscriber Details</DialogTitle>
      <DialogContent dividers>
        <Collapse in={!!alert.message}>
          <Alert
            onClose={() => setAlert({ message: "" })}
            sx={{ mb: 2 }}
            severity="info"
          >
            {alert.message}
          </Alert>
        </Collapse>
        <Box sx={{ mb: 2, display: "flex", alignItems: "center" }}>
          <Typography sx={{ flex: 1 }}>
            <strong>IMSI:</strong> {subscriberData.imsi}
          </Typography>
          <IconButton
            onClick={() => handleCopy(subscriberData.imsi, "IMSI")}
            aria-label="Copy IMSI"
          >
            <CopyIcon />
          </IconButton>
        </Box>
        <Box sx={{ mb: 2, display: "flex", alignItems: "center" }}>
          <Typography sx={{ flex: 1 }}>
            <strong>Key:</strong>{" "}
            {keyObfuscated
              ? "••••••••••••••••••••••••••••••••"
              : subscriberData.key}
          </Typography>
          <Button
            variant="text"
            onClick={() => setKeyObfuscated(!keyObfuscated)}
          >
            {keyObfuscated ? "Show" : "Hide"}
          </Button>
          <IconButton
            onClick={() => handleCopy(subscriberData.key, "Key")}
            aria-label="Copy Key"
          >
            <CopyIcon />
          </IconButton>
        </Box>
        <Box sx={{ mb: 2, display: "flex", alignItems: "center" }}>
          <Typography sx={{ flex: 1 }}>
            <strong>OPc:</strong>{" "}
            {opcObfuscated
              ? "••••••••••••••••••••••••••••••••"
              : subscriberData.opc}
          </Typography>
          <Button
            variant="text"
            onClick={() => setOPcObfuscated(!opcObfuscated)}
          >
            {opcObfuscated ? "Show" : "Hide"}
          </Button>
          <IconButton
            onClick={() => handleCopy(subscriberData.opc, "OPc")}
            aria-label="Copy OPc"
          >
            <CopyIcon />
          </IconButton>
        </Box>
        <Box sx={{ mb: 2, display: "flex", alignItems: "center" }}>
          <Typography sx={{ flex: 1 }}>
            <strong>Sequence Number:</strong> {subscriberData.sequenceNumber}
          </Typography>
          <IconButton
            onClick={() =>
              handleCopy(subscriberData.sequenceNumber, "Sequence Number")
            }
            aria-label="Copy Sequence Number"
          >
            <CopyIcon />
          </IconButton>
        </Box>
        <Box sx={{ mb: 2 }}>
          <Typography>
            <strong>Policy Name:</strong> {subscriberData.policyName}
          </Typography>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} sx={{ marginRight: 2 }}>
          Close
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ViewSubscriberModal;
