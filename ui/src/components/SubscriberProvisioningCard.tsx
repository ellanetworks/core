import React, { useState } from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  IconButton,
  Typography,
} from "@mui/material";
import { ContentCopy as CopyIcon, Edit as EditIcon } from "@mui/icons-material";
import { Link as RouterLink } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { useSnackbar } from "@/contexts/SnackbarContext";
import { useAuth } from "@/contexts/AuthContext";
import type { APISubscriber } from "@/queries/subscribers";
import theme from "@/utils/theme";
import { getSubscriberCredentials } from "@/queries/subscribers";

interface SubscriberProvisioningCardProps {
  subscriber: APISubscriber;
  onEditProfile?: () => void;
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
    <Typography
      variant="body2"
      sx={{
        flex: 1,
        wordBreak: "break-all",
        ...(linkTo
          ? {
              color: theme.palette.link,
              textDecoration: "underline",
              "&:hover": { textDecoration: "underline" },
            }
          : {}),
      }}
      {...(linkTo
        ? {
            component: RouterLink,
            to: linkTo,
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
    {copyable && onCopy ? (
      <IconButton size="small" onClick={onCopy} aria-label={`Copy ${label}`}>
        <CopyIcon fontSize="small" />
      </IconButton>
    ) : onCopy ? (
      <IconButton size="small" sx={{ visibility: "hidden" }} aria-hidden>
        <CopyIcon fontSize="small" />
      </IconButton>
    ) : null}
    {actionIcon}
  </Box>
);

const SubscriberProvisioningCard: React.FC<SubscriberProvisioningCardProps> = ({
  subscriber,
  onEditProfile,
}) => {
  const { showSnackbar } = useSnackbar();
  const { role, accessToken, authReady } = useAuth();
  const [credentialsVisible, setCredentialsVisible] = useState(false);

  const canViewCredentials = role === "Admin" || role === "Network Manager";

  const [credentialsRequested, setCredentialsRequested] = useState(false);

  const { data: credentials } = useQuery({
    queryKey: ["subscriberCredentials", subscriber.imsi],
    queryFn: () => getSubscriberCredentials(accessToken!, subscriber.imsi),
    enabled:
      authReady && !!accessToken && canViewCredentials && credentialsRequested,
  });

  const handleToggleCredentials = () => {
    if (!credentialsVisible) {
      setCredentialsRequested(true);
    }
    setCredentialsVisible((v) => !v);
  };

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
    <Card
      variant="outlined"
      sx={{ height: "100%", display: "flex", flexDirection: "column" }}
    >
      <CardContent sx={{ flex: 1, display: "flex", flexDirection: "column" }}>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            mb: 1.5,
          }}
        >
          <Typography variant="h6">Provisioning</Typography>
          {canViewCredentials && (
            <Button
              variant="text"
              size="small"
              onClick={handleToggleCredentials}
            >
              {credentialsVisible ? "Hide credentials" : "Show credentials"}
            </Button>
          )}
        </Box>
        <FieldRow
          label="Key"
          value={credentials?.key ?? ""}
          copyable={
            canViewCredentials && credentialsVisible && !!credentials?.key
          }
          onCopy={() => handleCopy(credentials?.key ?? "", "Key")}
          obfuscated={!credentialsVisible}
        />
        <FieldRow
          label="OPc"
          value={credentials?.opc ?? ""}
          copyable={
            canViewCredentials && credentialsVisible && !!credentials?.opc
          }
          onCopy={() => handleCopy(credentials?.opc ?? "", "OPc")}
          obfuscated={!credentialsVisible}
        />
        <FieldRow
          label="Sequence Number"
          value={credentials?.sequenceNumber ?? ""}
          copyable={
            canViewCredentials &&
            credentialsVisible &&
            !!credentials?.sequenceNumber
          }
          onCopy={() =>
            handleCopy(credentials?.sequenceNumber ?? "", "Sequence Number")
          }
          obfuscated={!credentialsVisible}
        />
        <FieldRow
          label="Profile"
          value={subscriber.profile_name || "—"}
          linkTo={`/profiles/${subscriber.profile_name}`}
          actionIcon={
            onEditProfile ? (
              <IconButton
                size="small"
                onClick={onEditProfile}
                aria-label="Edit profile"
                color="primary"
              >
                <EditIcon fontSize="small" />
              </IconButton>
            ) : undefined
          }
        />
      </CardContent>
    </Card>
  );
};

export default SubscriberProvisioningCard;
