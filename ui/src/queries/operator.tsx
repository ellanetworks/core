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

export type IMSVoiceStatus =
  "enabled" | "no_pcscf" | "no_ims_data_network" | "ip_family_mismatch";

export interface OperatorIMS {
  pcscfAddresses: string[];
  voiceStatus: IMSVoiceStatus;
  homeDomain: string;
}

export const MAX_PCSCF_ADDRESSES_PER_FAMILY = 3;

export const DEFAULT_SMSC_PORT = 3868;

export const MAX_SMSC_SERVICE_CENTRES = 16;

export const MAX_SMSC_NODES = 8;

export interface SMSCNodeInput {
  diameterIdentity: string;
  address: string;
  port: number;
}

export type SMSCNodeState =
  "down" | "connecting" | "open" | "suspect" | "reopen" | "closing";

export interface SMSCNode extends SMSCNodeInput {
  status?: {
    state: SMSCNodeState;
    host?: string;
    realm?: string;
    since: string;
    error?: string;
  };
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

export const listSMSCs = async (authToken: string): Promise<SMSC[]> => {
  const res = await apiFetch<{ items: SMSC[] }>(`/api/v1/operator/sms/smscs`, {
    authToken,
  });
  return res.items;
};

export const createSMSC = async (
  authToken: string,
  smsc: SMSCInput,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/sms/smscs`, {
    method: "POST",
    authToken,
    body: smsc,
  });
};

export const updateSMSC = async (
  authToken: string,
  name: string,
  smsc: SMSCUpdate,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/sms/smscs/${encodeURIComponent(name)}`, {
    method: "PUT",
    authToken,
    body: smsc,
  });
};

export const deleteSMSC = async (
  authToken: string,
  name: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/sms/smscs/${encodeURIComponent(name)}`, {
    method: "DELETE",
    authToken,
  });
};

export const updateOperatorIMS = async (
  authToken: string,
  ims: Pick<OperatorIMS, "pcscfAddresses">,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/operator/ims`, {
    method: "PUT",
    authToken,
    body: { pcscfAddresses: ims.pcscfAddresses },
  });
};
