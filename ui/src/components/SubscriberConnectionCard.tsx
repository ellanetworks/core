import React from "react";
import { Box, Card, CardContent, Typography } from "@mui/material";
import { Link as RouterLink } from "react-router-dom";
import SignalWifiOffIcon from "@mui/icons-material/SignalWifiOff";
import NorthIcon from "@mui/icons-material/North";
import SouthIcon from "@mui/icons-material/South";
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

const BitrateValue: React.FC<{ uplink?: string; downlink?: string }> = ({
  uplink,
  downlink,
}) => {
  if (!uplink && !downlink) return <Typography variant="body2">—</Typography>;
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.25 }}>
        <NorthIcon sx={{ fontSize: 16, color: "#FF9800" }} />
        <Typography variant="body2">{uplink || "—"}</Typography>
      </Box>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.25 }}>
        <SouthIcon sx={{ fontSize: 16, color: "#4254FB" }} />
        <Typography variant="body2">{downlink || "—"}</Typography>
      </Box>
    </Box>
  );
};

const CIPHERING_SHORT: Record<string, string> = {
  NEA0: "NEA0 (Null)",
  NEA1: "NEA1 (SNOW 3G)",
  NEA2: "NEA2 (AES)",
  NEA3: "NEA3 (ZUC)",
};

const INTEGRITY_SHORT: Record<string, string> = {
  NIA0: "NIA0 (Null)",
  NIA1: "NIA1 (SNOW 3G)",
  NIA2: "NIA2 (AES)",
  NIA3: "NIA3 (ZUC)",
};

const formatAlgorithm = (
  alg?: string,
  descriptions?: Record<string, string>,
): string => {
  if (!alg) return "";
  return descriptions?.[alg] ?? alg;
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
                Bitrate
              </Typography>
              <BitrateValue
                uplink={status.ambrUplink}
                downlink={status.ambrDownlink}
              />
            </Box>

            {/* Radio section */}
            <Typography
              variant="overline"
              sx={{
                display: "block",
                mt: 1.5,
                mb: 0.5,
                color: "text.primary",
                letterSpacing: 1.2,
              }}
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
              variant="overline"
              sx={{
                display: "block",
                mt: 1.5,
                mb: 0.5,
                color: "text.primary",
                letterSpacing: 1.2,
              }}
            >
              Security
            </Typography>
            <InfoRow
              label="Ciphering"
              value={formatAlgorithm(
                status.cipheringAlgorithm,
                CIPHERING_SHORT,
              )}
            />
            <InfoRow
              label="Integrity"
              value={formatAlgorithm(
                status.integrityAlgorithm,
                INTEGRITY_SHORT,
              )}
            />
          </>
        )}
      </CardContent>
    </Card>
  );
};

export default SubscriberConnectionCard;
