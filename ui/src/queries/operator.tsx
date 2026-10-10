// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export interface HomeNetworkKey {
  id: number;
  keyIdentifier: number;
  scheme: "A" | "B";
  publicKey: string;
}

export interface OperatorData {
  id: { mcc: string; mnc: string };
  tracking: { supportedTacs: string[] };
  homeNetworkKeys: HomeNetworkKey[];
  nasSecurity: {
    ciphering: string[];
    integrity: string[];
  };
  spn: { fullName: string; shortName: string };
  sms: OperatorSMS;
  ims: OperatorIMS;
}

export interface OperatorIMS {
  pcscfAddresses: string[];
}

export const MAX_PCSCF_ADDRESSES_PER_FAMILY = 3;

export const DEFAULT_SMSC_PORT = 3868;

export interface SMSCPeerInput {
  diameterIdentity: string;
  address: string;
  port: number;
  serviceCentres: string[];
}

export type SMSCPeerState =
  "down" | "connecting" | "open" | "suspect" | "reopen" | "closing";

export interface SMSCPeer extends SMSCPeerInput {
  id: string;
  status?: {
    state: SMSCPeerState;
    host?: string;
    realm?: string;
    since: string;
    error?: string;
  };
}

export interface OperatorSMS {
  smsNumber: string;
}

export const getOperator = async (authToken: string): Promise<OperatorData> => {
  return apiFetch<OperatorData>(`/api/v1/operator`, { authToken });
};

export const updateOperatorID = async (
  authToken: string,
  mcc: string,
  mnc: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/id`, {
    method: "PUT",
    authToken,
    body: { mcc, mnc },
  });
};

export const updateOperatorTracking = async (
  authToken: string,
  supportedTacs: string[],
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/tracking`, {
    method: "PUT",
    authToken,
    body: { supportedTacs },
  });
};

export const updateOperatorCode = async (
  authToken: string,
  operatorCode: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/code`, {
    method: "PUT",
    authToken,
    body: { operatorCode },
  });
};

export const createHomeNetworkKey = async (
  authToken: string,
  keyIdentifier: number,
  scheme: string,
  privateKey: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/home-network-keys`, {
    method: "POST",
    authToken,
    body: { keyIdentifier, scheme, privateKey },
  });
};

export const deleteHomeNetworkKey = async (
  authToken: string,
  id: number,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/home-network-keys/${id}`, {
    method: "DELETE",
    authToken,
  });
};

export const getHomeNetworkKeyPrivateKey = async (
  authToken: string,
  id: number,
): Promise<{ privateKey: string }> => {
  return apiFetch<{ privateKey: string }>(
    `/api/v1/operator/home-network-keys/${id}/private-key`,
    { authToken },
  );
};

export const updateOperatorNASSecurity = async (
  authToken: string,
  ciphering: string[],
  integrity: string[],
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/nas-security`, {
    method: "PUT",
    authToken,
    body: { ciphering, integrity },
  });
};

export const updateOperatorSPN = async (
  authToken: string,
  fullName: string,
  shortName: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/spn`, {
    method: "PUT",
    authToken,
    body: { fullName, shortName },
  });
};

export const updateOperatorSMS = async (
  authToken: string,
  sms: { smsNumber: string },
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/sms`, {
    method: "PUT",
    authToken,
    body: { smsNumber: sms.smsNumber },
  });
};

export const listSMSCPeers = async (authToken: string): Promise<SMSCPeer[]> => {
  const res = await apiFetch<{ items: SMSCPeer[] }>(
    `/api/v1/operator/sms/smsc-peers`,
    { authToken },
  );
  return res.items;
};

export const createSMSCPeer = async (
  authToken: string,
  peer: SMSCPeerInput,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/sms/smsc-peers`, {
    method: "POST",
    authToken,
    body: peer,
  });
};

export const updateSMSCPeer = async (
  authToken: string,
  id: string,
  peer: SMSCPeerInput,
): Promise<void> => {
  await apiFetchVoid(
    `/api/v1/operator/sms/smsc-peers/${encodeURIComponent(id)}`,
    { method: "PUT", authToken, body: peer },
  );
};

export const deleteSMSCPeer = async (
  authToken: string,
  id: string,
): Promise<void> => {
  await apiFetchVoid(
    `/api/v1/operator/sms/smsc-peers/${encodeURIComponent(id)}`,
    { method: "DELETE", authToken },
  );
};

export const updateOperatorIMS = async (
  authToken: string,
  ims: OperatorIMS,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/ims`, {
    method: "PUT",
    authToken,
    body: { pcscfAddresses: ims.pcscfAddresses },
  });
};
