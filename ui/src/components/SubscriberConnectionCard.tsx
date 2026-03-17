import React from "react";
import {
  Box,
  Card,
  CardContent,
  Chip,
  Divider,
  Typography,
  CircularProgress,
} from "@mui/material";
import WarningAmberIcon from "@mui/icons-material/WarningAmber";
import { Link as RouterLink } from "react-router-dom";
import type {
  SubscriberDetailStatus,
  SessionInfo,
} from "@/queries/subscribers";
import { formatRelativeTime } from "@/utils/formatters";

interface SubscriberConnectionCardProps {
  status: SubscriberDetailStatus;
  sessions?: SessionInfo[];
  loading?: boolean;
  ipAddress?: string | null;
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

const CIPHERING_LABELS: Record<string, string> = {
  NEA0: "NEA0",
  NEA1: "NEA1",
  NEA2: "NEA2",
  NEA3: "NEA3",
};

const INTEGRITY_LABELS: Record<string, string> = {
  NIA0: "NIA0",
  NIA1: "NIA1",
  NIA2: "NIA2",
  NIA3: "NIA3",
};

/** NEA0 / NIA0 are null ciphering/integrity — highlight as warning. */
const INSECURE_ALGS = new Set(["NEA0", "NIA0"]);

const AlgorithmChip: React.FC<{
  kind: string;
  alg?: string;
  labels: Record<string, string>;
}> = ({ kind, alg, labels }) => {
  if (!alg) return null;
  const display = labels[alg] ?? alg;
  const isInsecure = INSECURE_ALGS.has(alg);
  return (
    <Chip
      size="small"
      icon={
        isInsecure ? (
          <WarningAmberIcon sx={{ fontSize: 14, color: "#fff" }} />
        ) : undefined
      }
      label={
        <Box component="span" sx={{ display: "inline-flex", gap: 0.5 }}>
          <Box component="span" sx={{ opacity: 0.85, fontWeight: 400 }}>
            {kind}:
          </Box>
          <Box component="span">{display}</Box>
        </Box>
      }
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

const StateChip: React.FC<{ registered?: boolean }> = ({ registered }) => {
  const label = registered ? "Registered" : "Deregistered";
  return (
    <Chip
      size="small"
      label={label}
      color={registered ? "success" : "default"}
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

const StatusChip: React.FC<{ status?: string }> = ({ status }) => {
  const isActive = status?.toLowerCase() === "active";
  return (
    <Chip
      size="small"
      label={status || "—"}
      color={isActive ? "success" : "default"}
      variant="filled"
    />
  );
};

const SessionRow: React.FC<{
  label: string;
  value?: React.ReactNode;
}> = ({ label, value }) => {
  const isEmpty = value === undefined || value === "" || value === null;
  const display = isEmpty ? "—" : value;

  return (
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
      {typeof display === "string" || typeof display === "number" ? (
        <Typography variant="body2">{display}</Typography>
      ) : (
        display
      )}
    </Box>
  );
};

const SecurityAlgorithmsValue: React.FC<{
  ciphering?: string;
  integrity?: string;
}> = ({ ciphering, integrity }) => {
  if (!ciphering && !integrity)
    return <Typography variant="body2">—</Typography>;
  return (
    <Box
      sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}
    >
      {ciphering && (
        <AlgorithmChip
          kind="Ciphering"
          alg={ciphering}
          labels={CIPHERING_LABELS}
        />
      )}
      {integrity && (
        <AlgorithmChip
          kind="Integrity"
          alg={integrity}
          labels={INTEGRITY_LABELS}
        />
      )}
    </Box>
  );
};

const SubscriberConnectionCard: React.FC<SubscriberConnectionCardProps> = ({
  status,
  sessions = [],
  loading = false,
  ipAddress,
}) => {
  return (
    <Card
      variant="outlined"
      sx={{ height: "100%", display: "flex", flexDirection: "column" }}
    >
      <CardContent sx={{ flex: 1, display: "flex", flexDirection: "column" }}>
        <Typography variant="h6" sx={{ mb: 1.5 }}>
          Connection
        </Typography>
        <InfoRow
          label="State"
          value={<StateChip registered={status.registered} />}
        />
        <InfoRow label="IMEI" value={status.imei} />
        <InfoRow
          label="Last Seen"
          value={
            status.lastSeenAt ? (
              <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                <Typography variant="body2">
                  {formatRelativeTime(status.lastSeenAt)}
                </Typography>
                {status.lastSeenRadio && (
                  <>
                    <Typography
                      variant="body2"
                      sx={{ color: "text.secondary" }}
                    >
                      on
                    </Typography>
                    <Typography
                      variant="body2"
                      component={RouterLink}
                      to="/radios"
                      sx={{
                        color: (theme) => theme.palette.link,
                        textDecoration: "underline",
                        "&:hover": { textDecoration: "underline" },
                      }}
                    >
                      {status.lastSeenRadio}
                    </Typography>
                  </>
                )}
              </Box>
            ) : undefined
          }
        />
        <InfoRow
          label="Security Algorithms"
          value={
            <SecurityAlgorithmsValue
              ciphering={status.cipheringAlgorithm}
              integrity={status.integrityAlgorithm}
            />
          }
        />

        {(() => {
          if (loading) {
            return (
              <InfoRow
                label="PDU Session"
                value={
                  <Box sx={{ display: "flex", alignItems: "center" }}>
                    <Chip label={<CircularProgress size={16} />} />
                  </Box>
                }
              />
            );
          }

          if (sessions.length === 0) {
            return (
              <InfoRow
                label="PDU Session"
                value={
                  <Typography variant="body2" sx={{ color: "text.secondary" }}>
                    No active session
                  </Typography>
                }
              />
            );
          }

          const session = sessions[0];
          const ip = session.ipAddress ?? ipAddress ?? "—";

          return (
            <InfoRow
              label="PDU Session"
              value={
                <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                  <StatusChip status={session.status} />
                  <Typography variant="body2" sx={{ color: "text.secondary" }}>
                    /
                  </Typography>
                  <Typography variant="body2">{ip}</Typography>
                </Box>
              }
            />
          );
        })()}
      </CardContent>
    </Card>
  );
};

export default SubscriberConnectionCard;
