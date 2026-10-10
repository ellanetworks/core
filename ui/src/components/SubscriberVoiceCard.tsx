// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import React from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Typography,
} from "@mui/material";
import { Link as RouterLink } from "react-router-dom";
import type {
  APISubscriber,
  IMSPublicIdentity,
  IMSUserState,
} from "@/queries/subscribers";
import { getOperator } from "@/queries/operator";
import { listPolicies } from "@/queries/policies";
import { useAuth } from "@/contexts/AuthContext";
import { IMS_DATA_NETWORK } from "@/utils/voice";

const POLICIES_PER_PAGE = 100;

const userStates: Record<
  IMSUserState,
  { label: string; color: "success" | "info" | "warning" | "default" }
> = {
  registered: { label: "Registered", color: "success" },
  registered_unreg_services: {
    label: "Unregistered services",
    color: "info",
  },
  authentication_pending: {
    label: "Authenticating",
    color: "warning",
  },
  not_registered: { label: "Not registered", color: "default" },
};

const linkSx = {
  color: (theme: { palette: { link: string } }) => theme.palette.link,
  textDecoration: "underline",
  "&:hover": { textDecoration: "underline" },
};

const Row: React.FC<{ label: string; children: React.ReactNode }> = ({
  label,
  children,
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
    <Box sx={{ minWidth: 0, wordBreak: "break-word" }}>{children}</Box>
  </Box>
);

const PublicIdentity: React.FC<{ identity: IMSPublicIdentity }> = ({
  identity,
}) => {
  const state = userStates[identity.user_state] ?? {
    label: identity.user_state,
    color: "default" as const,
  };

  return (
    <Box
      sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}
    >
      <Typography variant="body2">{identity.identity}</Typography>
      <Chip
        label={state.label}
        color={state.color}
        size="small"
        variant="outlined"
      />
      {identity.barred && (
        <Chip label="Barred" size="small" variant="outlined" />
      )}
    </Box>
  );
};

const hasVoicePolicy = async (authToken: string, profileName: string) => {
  for (let page = 1; ; page++) {
    const result = await listPolicies(
      authToken,
      page,
      POLICIES_PER_PAGE,
      profileName,
    );
    const items = result.items ?? [];

    if (items.some((p) => p.data_network_name === IMS_DATA_NETWORK)) {
      return true;
    }

    if (
      items.length < POLICIES_PER_PAGE ||
      page * POLICIES_PER_PAGE >= (result.total_count ?? 0)
    ) {
      return false;
    }
  }
};

interface SubscriberVoiceCardProps {
  subscriber: APISubscriber;
  onEditMSISDN?: () => void;
}

const SubscriberVoiceCard: React.FC<SubscriberVoiceCardProps> = ({
  subscriber,
  onEditMSISDN,
}) => {
  const { accessToken, authReady } = useAuth();
  const enabled = authReady && !!accessToken;

  const policyQuery = useQuery({
    queryKey: ["voice-policy", subscriber.profile_name],
    queryFn: () => hasVoicePolicy(accessToken!, subscriber.profile_name),
    enabled,
  });

  const operatorQuery = useQuery({
    queryKey: ["operator"],
    queryFn: () => getOperator(accessToken!),
    enabled,
  });

  const loading = !policyQuery.data && policyQuery.isLoading;
  const missing: React.ReactNode[] = [];

  if (!subscriber.msisdn) {
    missing.push(
      onEditMSISDN ? (
        <Button
          key="msisdn"
          size="small"
          onClick={onEditMSISDN}
          sx={{ p: 0, minWidth: 0, textTransform: "none", ...linkSx }}
        >
          a phone number
        </Button>
      ) : (
        "a phone number"
      ),
    );
  }

  if (policyQuery.data === false) {
    missing.push(
      <Typography
        key="policy"
        variant="body2"
        component={RouterLink}
        to={`/profiles/${encodeURIComponent(subscriber.profile_name)}`}
        sx={linkSx}
      >
        a policy on {IMS_DATA_NETWORK} in its profile
      </Typography>,
    );
  }

  if (
    operatorQuery.data &&
    operatorQuery.data.ims.pcscfAddresses.length === 0
  ) {
    missing.push(
      <Typography
        key="pcscf"
        variant="body2"
        component={RouterLink}
        to="/operator"
        sx={linkSx}
      >
        P-CSCF addresses
      </Typography>,
    );
  }

  const voPS = subscriber.registrations.filter(
    (r) => r.ims_voice_over_ps !== undefined,
  );
  const ims = subscriber.ims;

  return (
    <Card variant="outlined">
      <CardContent>
        <Typography variant="h6" sx={{ mb: 1.5 }}>
          Voice
        </Typography>
        <Row label="Setup">
          {loading ? (
            <Typography variant="body2" color="textSecondary">
              Checking…
            </Typography>
          ) : missing.length === 0 ? (
            <Chip
              label="Ready"
              color="success"
              size="small"
              variant="outlined"
            />
          ) : (
            <Typography variant="body2" component="div">
              Needs{" "}
              {missing.map((item, i) => (
                <React.Fragment key={i}>
                  {i > 0 && (i === missing.length - 1 ? " and " : ", ")}
                  {item}
                </React.Fragment>
              ))}
              .
            </Typography>
          )}
        </Row>
        {voPS.map((r) => (
          <Row key={r.system} label={`IMS Voice over PS (${r.system})`}>
            <Chip
              label={r.ims_voice_over_ps ? "Supported" : "Not supported"}
              color={r.ims_voice_over_ps ? "success" : "default"}
              size="small"
              variant="outlined"
            />
          </Row>
        ))}
        {ims && (
          <>
            <Row label="Private Identity">
              <Typography variant="body2">{ims.private_identity}</Typography>
            </Row>
            <Row label="Public Identities">
              <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
                {ims.public_identities.map((identity) => (
                  <PublicIdentity key={identity.identity} identity={identity} />
                ))}
              </Box>
            </Row>
            <Row label="S-CSCF">
              <Typography variant="body2">{ims.scscf_name ?? "—"}</Typography>
            </Row>
          </>
        )}
      </CardContent>
    </Card>
  );
};

export default SubscriberVoiceCard;
