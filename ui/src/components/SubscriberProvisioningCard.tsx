import React, { useState } from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  IconButton,
  Typography,
} from "@mui/material";
import {
  ContentCopy as CopyIcon,
  Edit as EditIcon,
  North as NorthIcon,
  South as SouthIcon,
} from "@mui/icons-material";
import { Link as RouterLink } from "react-router-dom";
import { useSnackbar } from "@/contexts/SnackbarContext";
import type { APISubscriber } from "@/queries/subscribers";
import { UPLINK_COLOR, DOWNLINK_COLOR } from "@/utils/formatters";

interface SubscriberProvisioningCardProps {
  subscriber: APISubscriber;
  onEditPolicy?: () => void;
}

const DOTS = "••••••••••••••••••••••••••••••••";

const FieldRow: React.FC<{
  label: string;
  value: string;
  copyable?: boolean;
  onCopy?: () => void;
  obfuscated?: boolean;
  onToggle?: () => void;
  linkTo?: string;
  actionIcon?: React.ReactNode;
}> = ({
  label,
  value,
  copyable,
  onCopy,
  obfuscated,
  onToggle,
  linkTo,
  actionIcon,
}) => (
  <Box
    sx={{
      display: "flex",
      alignItems: "center",
      py: 0.75,
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
    <Typography
      variant="body2"
      sx={{
        flex: 1,
        wordBreak: "break-all",
      }}
      {...(linkTo
        ? {
            component: RouterLink,
            to: linkTo,
            color: "primary",
          }
        : {})}
    >
      {obfuscated ? DOTS : value || "—"}
    </Typography>
    {onToggle && (
      <Button
        variant="text"
        size="small"
        onClick={onToggle}
        sx={{ minWidth: 56 }}
      >
        {obfuscated ? "Show" : "Hide"}
      </Button>
    )}
    {copyable && onCopy && (
      <IconButton size="small" onClick={onCopy} aria-label={`Copy ${label}`}>
        <CopyIcon fontSize="small" />
      </IconButton>
    )}
    {actionIcon}
  </Box>
);

const SubscriberProvisioningCard: React.FC<SubscriberProvisioningCardProps> = ({
  subscriber,
  onEditPolicy,
}) => {
  const { showSnackbar } = useSnackbar();
  const [keyObfuscated, setKeyObfuscated] = useState(true);
  const [opcObfuscated, setOpcObfuscated] = useState(true);

  const handleCopy = async (value: string, label: string) => {
    if (!navigator.clipboard) {
      showSnackbar(
        "Clipboard API not available. Please use HTTPS or try a different browser.",
        "error",
      );
      return;
    }
    try {
      await navigator.clipboard.writeText(value);
      showSnackbar("Copied to clipboard.", "success");
    } catch {
      showSnackbar(`Failed to copy ${label}.`, "error");
    }
  };

  return (
    <Card variant="outlined" sx={{ height: "100%" }}>
      <CardContent>
        <Typography variant="h6" sx={{ mb: 1.5 }}>
          Provisioning
        </Typography>
        <FieldRow
          label="Key"
          value={subscriber.key}
          copyable
          onCopy={() => handleCopy(subscriber.key, "Key")}
          obfuscated={keyObfuscated}
          onToggle={() => setKeyObfuscated((v) => !v)}
        />
        <FieldRow
          label="OPc"
          value={subscriber.opc}
          copyable
          onCopy={() => handleCopy(subscriber.opc, "OPc")}
          obfuscated={opcObfuscated}
          onToggle={() => setOpcObfuscated((v) => !v)}
        />
        <FieldRow
          label="Sequence Number"
          value={subscriber.sequenceNumber}
          copyable
          onCopy={() =>
            handleCopy(subscriber.sequenceNumber, "Sequence Number")
          }
        />
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            py: 0.75,
            "&:not(:last-child)": {
              borderBottom: "1px solid",
              borderColor: "divider",
            },
          }}
        >
          <Typography
            variant="body2"
            sx={{
              color: "text.secondary",
              minWidth: 180,
              flexShrink: 0,
              mr: 2,
            }}
          >
            Policy
          </Typography>
          <Typography
            variant="body2"
            component={RouterLink}
            to="/policies"
            sx={{
              color: "primary.main",
              textDecoration: "none",
              "&:hover": { textDecoration: "underline" },
            }}
          >
            {subscriber.policyName}
          </Typography>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, ml: 2 }}>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.25 }}>
              <SouthIcon sx={{ fontSize: 14, color: DOWNLINK_COLOR }} />
              <Typography variant="caption" color="text.secondary">
                {subscriber.policyBitrateDownlink || "—"}
              </Typography>
            </Box>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.25 }}>
              <NorthIcon sx={{ fontSize: 14, color: UPLINK_COLOR }} />
              <Typography variant="caption" color="text.secondary">
                {subscriber.policyBitrateUplink || "—"}
              </Typography>
            </Box>
          </Box>
          <Box sx={{ flex: 1 }} />
          {onEditPolicy && (
            <IconButton
              size="small"
              onClick={onEditPolicy}
              aria-label="Edit policy"
              color="primary"
            >
              <EditIcon fontSize="small" />
            </IconButton>
          )}
        </Box>
        <FieldRow
          label="Data Network"
          value={subscriber.dataNetworkName || "—"}
          linkTo="/networking?tab=data-networks"
        />
      </CardContent>
    </Card>
  );
};

export default SubscriberProvisioningCard;
