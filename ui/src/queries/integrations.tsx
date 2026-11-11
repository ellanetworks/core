// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type DiameterPeerState =
  "down" | "connecting" | "open" | "suspect" | "reopen" | "closing";

export interface DiameterPeerStatus {
  state: DiameterPeerState;
  originHost?: string;
  realm?: string;
  since: string;
  error?: string;
}

export interface SMSSettings {
  servingNodeNumber: string;
}

export const DEFAULT_SMSC_PORT = 3868;

export const MAX_SMSC_SERVICE_CENTRES = 16;

export const MAX_SMSC_NODES = 8;

export interface SMSCNodeInput {
  originHost: string;
  address: string;
  port: number;
}

export interface SMSCNode extends SMSCNodeInput {
  status?: DiameterPeerStatus;
}

export interface SMSCUpdate {
  serviceCentres: string[];
  nodes: SMSCNodeInput[];
}

export interface SMSCInput extends SMSCUpdate {
  name: string;
}

export interface SMSC {
  name: string;
  serviceCentres: string[];
  nodes: SMSCNode[];
}

export type IMSVoiceStatus =
  "enabled" | "no_pcscf" | "no_ims_data_network" | "ip_family_mismatch";

export interface IMSSettings {
  pcscfAddresses: string[];
  voiceStatus: IMSVoiceStatus;
  homeDomain: string;
  derivedHomeDomain: string;
  advertiseVoice: boolean;
  restrictPeers: boolean;
}

export interface IMSSettingsUpdate {
  pcscfAddresses: string[];
  homeDomain: string;
  advertiseVoice: boolean;
  restrictPeers: boolean;
}

export const MAX_PCSCF_ADDRESSES_PER_FAMILY = 5;

export const MAX_PCSCF_ADDRESSES = 2 * MAX_PCSCF_ADDRESSES_PER_FAMILY;

export const primaryHomeDomain = (
  ims?: Pick<IMSSettings, "homeDomain" | "derivedHomeDomain"> | null,
): string => ims?.homeDomain || ims?.derivedHomeDomain || "";

export interface IMSPeerInput {
  originHost: string;
  realm: string;
  addresses: string[];
}

export interface IMSPeer {
  name: string;
  originHost?: string;
  realm?: string;
  addresses: string[];
  connections: DiameterPeerStatus[];
}

export type DiameterPeerRole = "smsc" | "ims" | "roaming" | "unknown";

export type DiameterPeerOwnerKind = "smsc" | "imsPeer" | "roamingDiameterPeer";

export interface DiameterNodeIdentity {
  nodeId: string;
  displayName?: string;
  originHost: string;
  local: boolean;
}

export interface DiameterListener {
  enabled: boolean;
  addresses: string[];
  port?: number;
}

export interface DiameterPeer {
  id: string;
  role: DiameterPeerRole;
  owner?: { kind: DiameterPeerOwnerKind; name: string };
  originHost?: string;
  realm?: string;
  address: string;
  port?: number;
  direction: "inbound" | "outbound";
  state: DiameterPeerState;
  since: string;
  error?: string;
}

export interface DiameterStatus {
  realm?: string;
  servedRealms: string[];
  nodes: DiameterNodeIdentity[];
  listener: DiameterListener;
  peers: DiameterPeer[];
}

const INTEGRATIONS_PATH = "/api/v1/integrations";
const SMS_PATH = `${INTEGRATIONS_PATH}/sms`;
const SMSCS_PATH = `${SMS_PATH}/smscs`;
const IMS_PATH = `${INTEGRATIONS_PATH}/ims`;
const IMS_PEERS_PATH = `${IMS_PATH}/allowed-peers`;

export const getSMSSettings = async (authToken: string): Promise<SMSSettings> =>
  apiFetch<SMSSettings>(SMS_PATH, { authToken });

export const updateSMSSettings = async (
  authToken: string,
  settings: SMSSettings,
): Promise<void> => {
  await apiFetchVoid(SMS_PATH, {
    method: "PUT",
    authToken,
    body: { servingNodeNumber: settings.servingNodeNumber },
  });
};

export const listSMSCs = async (authToken: string): Promise<SMSC[]> => {
  const res = await apiFetch<{ items: SMSC[] }>(SMSCS_PATH, { authToken });
  return res?.items ?? [];
};

export const createSMSC = async (
  authToken: string,
  smsc: SMSCInput,
): Promise<void> => {
  await apiFetchVoid(SMSCS_PATH, { method: "POST", authToken, body: smsc });
};

export const updateSMSC = async (
  authToken: string,
  name: string,
  smsc: SMSCUpdate,
): Promise<void> => {
  await apiFetchVoid(`${SMSCS_PATH}/${encodeURIComponent(name)}`, {
    method: "PUT",
    authToken,
    body: smsc,
  });
};

export const deleteSMSC = async (
  authToken: string,
  name: string,
): Promise<void> => {
  await apiFetchVoid(`${SMSCS_PATH}/${encodeURIComponent(name)}`, {
    method: "DELETE",
    authToken,
  });
};

export const getIMSSettings = async (authToken: string): Promise<IMSSettings> =>
  apiFetch<IMSSettings>(IMS_PATH, { authToken });

export const updateIMSSettings = async (
  authToken: string,
  settings: IMSSettingsUpdate,
): Promise<void> => {
  await apiFetchVoid(IMS_PATH, {
    method: "PUT",
    authToken,
    body: {
      pcscfAddresses: settings.pcscfAddresses,
      homeDomain: settings.homeDomain,
      advertiseVoice: settings.advertiseVoice,
      restrictPeers: settings.restrictPeers,
    },
  });
};

export const listIMSPeers = async (authToken: string): Promise<IMSPeer[]> => {
  const res = await apiFetch<{ items: IMSPeer[] }>(IMS_PEERS_PATH, {
    authToken,
  });
  return res?.items ?? [];
};

export const createIMSPeer = async (
  authToken: string,
  peer: IMSPeerInput & { name: string },
): Promise<void> => {
  await apiFetchVoid(IMS_PEERS_PATH, { method: "POST", authToken, body: peer });
};

export const updateIMSPeer = async (
  authToken: string,
  name: string,
  peer: IMSPeerInput,
): Promise<void> => {
  await apiFetchVoid(`${IMS_PEERS_PATH}/${encodeURIComponent(name)}`, {
    method: "PUT",
    authToken,
    body: peer,
  });
};

export const deleteIMSPeer = async (
  authToken: string,
  name: string,
): Promise<void> => {
  await apiFetchVoid(`${IMS_PEERS_PATH}/${encodeURIComponent(name)}`, {
    method: "DELETE",
    authToken,
  });
};

export const getDiameterStatus = async (
  authToken: string,
): Promise<DiameterStatus> =>
  apiFetch<DiameterStatus>(`${INTEGRATIONS_PATH}/diameter`, { authToken });
