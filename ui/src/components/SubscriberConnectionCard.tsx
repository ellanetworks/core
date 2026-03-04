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

const CIPHERING_DESCRIPTIONS: Record<string, string> = {
  NEA0: "Null ciphering (no encryption)",
  NEA1: "SNOW 3G based encryption",
  NEA2: "AES based encryption",
  NEA3: "ZUC based encryption",
};

const INTEGRITY_DESCRIPTIONS: Record<string, string> = {
  NIA0: "Null integrity (no protection)",
  NIA1: "SNOW 3G based integrity",
  NIA2: "AES based integrity",
  NIA3: "ZUC based integrity",
};

const formatAlgorithm = (
  alg?: string,
  descriptions?: Record<string, string>,
): string => {
  if (!alg) return "";
  const desc = descriptions?.[alg];
  return desc ? `${alg} — ${desc}` : alg;
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
            <InfoRow label="IP Address" value={status.ipAddress} />
            <InfoRow label="State" value={status.state} />
            <InfoRow label="IMEI" value={status.imei} />
            <InfoRow
              label="Active Sessions"
              value={formatSessions(status.activeSessions)}
            />
            <InfoRow
              label="Bitrate"
              value={formatAmbr(status.ambrUplink, status.ambrDownlink)}
            />

            {/* Radio section */}
            <Typography
              variant="subtitle2"
              sx={{ mt: 2, mb: 0.5, color: "text.secondary" }}
            >
              Radio
            </Typography>
            <InfoRow
              label="Connected Radio"
              value={status.connectedRadio}
              linkTo="/radios"
            />
            <InfoRow label="TAC" value={status.tac} />
            <InfoRow label="Cell ID" value={status.cellID} />

            {/* Security section */}
            <Typography
              variant="subtitle2"
              sx={{ mt: 2, mb: 0.5, color: "text.secondary" }}
            >
              Security
            </Typography>
            <InfoRow
              label="Ciphering"
              value={formatAlgorithm(
                status.cipheringAlgorithm,
                CIPHERING_DESCRIPTIONS,
              )}
            />
            <InfoRow
              label="Integrity"
              value={formatAlgorithm(
                status.integrityAlgorithm,
                INTEGRITY_DESCRIPTIONS,
              )}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
};

export default SubscriberConnectionCard;
