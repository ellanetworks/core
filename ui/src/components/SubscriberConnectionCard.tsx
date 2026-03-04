import React from "react";
import { Box, Card, CardContent, Chip, Typography } from "@mui/material";
import { Link as RouterLink } from "react-router-dom";
import SignalWifiOffIcon from "@mui/icons-material/SignalWifiOff";
import NorthIcon from "@mui/icons-material/North";
import SouthIcon from "@mui/icons-material/South";
import type { SubscriberDetailStatus } from "@/queries/subscribers";

const UPLINK_COLOR = "#FF9800";
const DOWNLINK_COLOR = "#4254FB";

interface SubscriberConnectionCardProps {
  status: SubscriberDetailStatus;
}

const InfoRow: React.FC<{
  label: string;
  value?: React.ReactNode;
  linkTo?: string;
}> = ({ label, value, linkTo }) => {
  const isEmpty = value === undefined || value === "" || value === null;
  const display = isEmpty ? "—" : value;

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
      {linkTo && !isEmpty ? (
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
      ) : typeof display === "string" || typeof display === "number" ? (
        <Typography variant="body2">{display}</Typography>
      ) : (
        display
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

/** Downlink first, then uplink — consistent with chart totals. */
const BitrateValue: React.FC<{ uplink?: string; downlink?: string }> = ({
  uplink,
  downlink,
}) => {
  if (!uplink && !downlink) return <Typography variant="body2">—</Typography>;
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.25 }}>
        <SouthIcon sx={{ fontSize: 16, color: DOWNLINK_COLOR }} />
        <Typography variant="body2">{downlink || "—"}</Typography>
      </Box>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.25 }}>
        <NorthIcon sx={{ fontSize: 16, color: UPLINK_COLOR }} />
        <Typography variant="body2">{uplink || "—"}</Typography>
      </Box>
    </Box>
  );
};

const CIPHERING_LABELS: Record<string, string> = {
  NEA0: "NEA0 (Null)",
  NEA1: "NEA1 (SNOW 3G)",
  NEA2: "NEA2 (AES)",
  NEA3: "NEA3 (ZUC)",
};

const INTEGRITY_LABELS: Record<string, string> = {
  NIA0: "NIA0 (Null)",
  NIA1: "NIA1 (SNOW 3G)",
  NIA2: "NIA2 (AES)",
  NIA3: "NIA3 (ZUC)",
};

/** NEA0 / NIA0 are null ciphering/integrity — highlight as warning. */
const INSECURE_ALGS = new Set(["NEA0", "NIA0"]);

const AlgorithmChip: React.FC<{
  alg?: string;
  labels: Record<string, string>;
}> = ({ alg, labels }) => {
  if (!alg) return <Typography variant="body2">—</Typography>;
  const display = labels[alg] ?? alg;
  const isInsecure = INSECURE_ALGS.has(alg);
  return (
    <Chip
      size="small"
      label={display}
      sx={{
        fontWeight: 600,
        fontSize: "0.75rem",
        height: 22,
        ...(isInsecure
          ? { backgroundColor: "#F9A825", color: "#fff" }
          : { backgroundColor: "success.main", color: "#fff" }),
      }}
    />
  );
};

const StateChip: React.FC<{ state?: string }> = ({ state }) => {
  if (!state) return <Typography variant="body2">—</Typography>;
  const isRegistered = state === "Registered";
  return (
    <Chip
      size="small"
      label={state}
      color={isRegistered ? "success" : "default"}
      variant="filled"
    />
  );
};

const IpChip: React.FC<{ ip?: string }> = ({ ip }) => {
  if (!ip) return <Typography variant="body2">—</Typography>;
  return (
    <Chip
      size="small"
      label={ip}
      color="success"
      variant="filled"
      sx={{ fontSize: "0.75rem" }}
    />
  );
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
            <InfoRow label="State" value={<StateChip state={status.state} />} />
            <InfoRow label="IP Address" value={<IpChip ip={status.ipAddress} />} />
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
              value={
                <AlgorithmChip
                  alg={status.cipheringAlgorithm}
                  labels={CIPHERING_LABELS}
                />
              }
            />
            <InfoRow
              label="Integrity"
              value={
                <AlgorithmChip
                  alg={status.integrityAlgorithm}
                  labels={INTEGRITY_LABELS}
                />
              }
            />
          </>
        )}
      </CardContent>
    </Card>
  );
};

export default SubscriberConnectionCard;
