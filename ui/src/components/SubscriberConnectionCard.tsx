// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Typography,
} from "@mui/material";
import WarningAmberIcon from "@mui/icons-material/WarningAmber";
import { Link as RouterLink } from "react-router-dom";
import AccessChip from "@/components/AccessChip";
import type {
  ConnectionState,
  IMSSubscription,
  IMSUserState,
  IPSMGW,
  SubscriberDetailStatus,
} from "@/queries/subscribers";
import type { IMSSettings, IMSVoiceStatus } from "@/queries/integrations";
import { formatRelativeTime } from "@/utils/formatters";
import { IMS_DATA_NETWORK } from "@/utils/voice";
import { radioDisplayName, radioPath } from "@/queries/radios";

interface SubscriberConnectionCardProps {
  status: SubscriberDetailStatus;
  ims?: IMSSubscription;
  ipSMGW?: IPSMGW;
  network?: IMSSettings;
  onDeregisterIMS?: () => void;
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

/** NEA0/NIA0 (5G) and EEA0/EIA0 (4G) are null ciphering/integrity. */
const INSECURE_ALGS = new Set(["NEA0", "NIA0", "EEA0", "EIA0"]);

const AlgorithmChip: React.FC<{
  kind: string;
  alg?: string;
}> = ({ kind, alg }) => {
  if (!alg) return null;

  const isInsecure = INSECURE_ALGS.has(alg);

  return (
    <Chip
      size="small"
      icon={isInsecure ? <WarningAmberIcon sx={{ fontSize: 14 }} /> : undefined}
      label={
        <Box component="span" sx={{ display: "inline-flex", gap: 0.5 }}>
          <Box component="span" sx={{ opacity: 0.85, fontWeight: 400 }}>
            {kind}:
          </Box>
          <Box component="span">{alg}</Box>
        </Box>
      }
      sx={{
        fontWeight: 600,
        fontSize: "0.75rem",
        height: 22,
        ...(isInsecure
          ? {
              backgroundColor: "warning.main",
              color: "common.black",
              "& .MuiChip-icon": { color: "common.black" },
            }
          : {
              backgroundColor: "success.main",
              color: "success.contrastText",
            }),
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

const ConnectionChip: React.FC<{ state?: ConnectionState }> = ({ state }) => {
  if (!state) return null;

  const connected = state === "connected";

  return (
    <Chip
      size="small"
      label={connected ? "Connected" : "Idle"}
      color={connected ? "success" : "default"}
      variant="filled"
    />
  );
};

type ChipColor = "success" | "default" | "warning";

interface VoiceState {
  label: string;
  color: ChipColor;
  detail?: string;
}

const imsUserStates: Record<IMSUserState, VoiceState> = {
  registered: { label: "Registered", color: "success" },
  registered_unreg_services: { label: "Not registered", color: "default" },
  authentication_pending: { label: "Authenticating", color: "default" },
  not_registered: { label: "Not registered", color: "default" },
};

const networkReasons: Record<Exclude<IMSVoiceStatus, "enabled">, string> = {
  no_pcscf: "No P-CSCF address is set in Integrations › IMS",
  no_ims_data_network: `There is no IMS data network named ${IMS_DATA_NETWORK}`,
  ip_family_mismatch: `No P-CSCF address matches the ${IMS_DATA_NETWORK} data network's IP family`,
};

export const voiceState = (
  status: SubscriberDetailStatus,
  ims?: IMSSubscription,
  network?: IMSSettings,
): VoiceState | undefined => {
  if (!ims) {
    if (!network?.pcscfAddresses.length) return undefined;

    return {
      label: "Not subscribed",
      color: "default",
      detail: `The profile has no policy on the ${IMS_DATA_NETWORK} data network`,
    };
  }

  if (ims.public_identities.every((identity) => identity.barred)) {
    return {
      label: "No phone number",
      color: "warning",
      detail: "Set an MSISDN to give the subscriber voice",
    };
  }

  if (network?.voiceStatus && network.voiceStatus !== "enabled") {
    return {
      label: "Off",
      color: "warning",
      detail: networkReasons[network.voiceStatus],
    };
  }

  if (network?.advertiseVoice === false) {
    return {
      label: "Not offered to the phone",
      color: "warning",
      detail: "Advertise IMS Voice is off in Integrations › IMS",
    };
  }

  if (status.registered && status.ims_voice_over_ps === false) {
    return {
      label: "Not offered to the phone",
      color: "warning",
      detail: "Changes reach the phone at its next registration",
    };
  }

  const state =
    ims.public_identities.find((i) => i.registration_set === "mmtel")
      ?.user_state ?? "not_registered";

  return imsUserStates[state] ?? { label: state, color: "default" };
};

export const hasIMSRegistration = (
  ims?: IMSSubscription,
  ipSMGW?: IPSMGW,
): boolean =>
  !!ipSMGW ||
  !!ims?.public_identities.some(
    (identity) => identity.user_state !== "not_registered",
  );

export const mcpttState = (ims?: IMSSubscription): VoiceState | undefined => {
  const state = ims?.public_identities.find(
    (i) => i.registration_set === "mcptt",
  )?.user_state;

  if (!state) return undefined;

  return imsUserStates[state] ?? { label: state, color: "default" };
};

const VoiceValue: React.FC<{ state: VoiceState }> = ({ state }) => (
  <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
    <Chip
      label={state.label}
      color={state.color}
      size="small"
      variant="filled"
    />
    {state.detail && (
      <Typography variant="body2" sx={{ color: "text.secondary" }}>
        {state.detail}
      </Typography>
    )}
  </Box>
);

const SystemChips: React.FC<{ systems: string[] }> = ({ systems }) => (
  <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
    {systems.map((system) => (
      <AccessChip key={system} label={system} />
    ))}
  </Box>
);

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
      {ciphering && <AlgorithmChip kind="Ciphering" alg={ciphering} />}
      {integrity && <AlgorithmChip kind="Integrity" alg={integrity} />}
    </Box>
  );
};

const SubscriberConnectionCard: React.FC<SubscriberConnectionCardProps> = ({
  status,
  ims,
  ipSMGW,
  network,
  onDeregisterIMS,
}) => {
  const systems = status.systems ?? [];
  const voice = voiceState(status, ims, network);
  const mcptt = mcpttState(ims);

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
          <Typography variant="h6">Status</Typography>
          {onDeregisterIMS && hasIMSRegistration(ims, ipSMGW) && (
            <Button variant="text" size="small" onClick={onDeregisterIMS}>
              Deregister from IMS
            </Button>
          )}
        </Box>
        <InfoRow
          label="Registration"
          value={<StateChip registered={status.registered} />}
        />
        <InfoRow
          label="Connection"
          value={<ConnectionChip state={status.connection_state} />}
        />
        {voice && (
          <InfoRow label="Voice" value={<VoiceValue state={voice} />} />
        )}
        {mcptt && (
          <InfoRow label="MCPTT" value={<VoiceValue state={mcptt} />} />
        )}
        {systems.length > 0 && (
          <InfoRow
            label={systems.length > 1 ? "Systems" : "System"}
            value={<SystemChips systems={systems} />}
          />
        )}
        <InfoRow label="IMEI" value={status.imei} />
        <InfoRow
          label="Last Seen"
          value={
            status.last_seen_at ? (
              <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
                <Typography variant="body2">
                  {formatRelativeTime(status.last_seen_at)}
                </Typography>
                {status.last_seen_radio_ref && (
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
                      to={`/radios/${radioPath(status.last_seen_radio_ref)}`}
                      sx={{
                        color: (theme) => theme.palette.link,
                        textDecoration: "underline",
                        "&:hover": { textDecoration: "underline" },
                      }}
                    >
                      {radioDisplayName(
                        status.last_seen_radio,
                        status.last_seen_radio_ref,
                      )}
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
              ciphering={status.ciphering_algorithm}
              integrity={status.integrity_algorithm}
            />
          }
        />
      </CardContent>
    </Card>
  );
};

export default SubscriberConnectionCard;
