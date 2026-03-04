import React from "react";
import { Box, Card, CardContent, Typography } from "@mui/material";
import { Link as RouterLink } from "react-router-dom";
import SignalWifiOffIcon from "@mui/icons-material/SignalWifiOff";
import type { SubscriberStatus } from "@/queries/subscribers";

interface SubscriberConnectionCardProps {
  status: SubscriberStatus;
}

const InfoRow: React.FC<{
  label: string;
  value?: string | number;
  linkTo?: string;
}> = ({ label, value, linkTo }) => {
  const display = value === undefined || value === "" ? "—" : String(value);

  return (
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
        sx={{ color: "text.secondary", minWidth: 160, flexShrink: 0 }}
      >
        {label}
      </Typography>
      {linkTo && display !== "—" ? (
        <Typography
          variant="body2"
          component={RouterLink}
          to={linkTo}
          sx={{
            color: "primary.main",
            textDecoration: "none",
            "&:hover": { textDecoration: "underline" },
          }}
        >
          {display}
        </Typography>
      ) : (
        <Typography variant="body2">{display}</Typography>
      )}
    </Box>
  );
};

const OfflineState: React.FC = () => (
  <Box
    sx={{
      display: "flex",
      flexDirection: "column",
      alignItems: "center",
      justifyContent: "center",
      py: 4,
      color: "text.secondary",
    }}
  >
    <SignalWifiOffIcon sx={{ fontSize: 40, mb: 1, opacity: 0.5 }} />
    <Typography variant="body2">Subscriber is offline</Typography>
  </Box>
);

const formatSessions = (count?: number): string => {
  if (count === undefined || count === 0) return "0";
  return `${count} active`;
};

const formatAmbr = (uplink?: string, downlink?: string): string => {
  if (!uplink && !downlink) return "";
  return `↑ ${uplink || "—"}  ↓ ${downlink || "—"}`;
};

const SubscriberConnectionCard: React.FC<SubscriberConnectionCardProps> = ({
  status,
}) => {
  const isOnline =
    status.state !== undefined && status.state !== "Deregistered";

  return (
    <Card variant="outlined">
      <CardContent>
        <Typography variant="h6" sx={{ mb: 1.5 }}>
          Connection
        </Typography>
        {!isOnline ? (
          <OfflineState />
        ) : (
          <>
            <InfoRow
              label="Radio"
              value={status.connectedRadio}
              linkTo="/radios"
            />
            <InfoRow label="IP Address" value={status.ipAddress} />
            <InfoRow label="State" value={status.state} />
            <InfoRow label="PEI (IMEI)" value={status.pei} />
            <InfoRow label="TAC" value={status.tac} />
            <InfoRow label="Cell ID" value={status.cellID} />
            <InfoRow
              label="Active Sessions"
              value={formatSessions(status.activeSessions)}
            />
            <InfoRow
              label="AMBR"
              value={formatAmbr(status.ambrUplink, status.ambrDownlink)}
            />
            <InfoRow label="Ciphering" value={status.cipheringAlgorithm} />
            <InfoRow label="Integrity" value={status.integrityAlgorithm} />
          </>
        )}
      </CardContent>
    </Card>
  );
};

export default SubscriberConnectionCard;
