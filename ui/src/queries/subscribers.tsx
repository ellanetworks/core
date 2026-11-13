// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

// Mirrors MaxDescriptionLength in internal/api/server/api_subscribers.go.
export const MAX_DESCRIPTION_LENGTH = 64;

export type ConnectionState = "idle" | "connected";

export type SubscriberListStatus = {
  registered?: boolean;
  connection_state?: ConnectionState;
  systems?: System[];
  num_sessions?: number;
  last_seen_at?: string;
  last_seen_radio?: string;
  last_seen_radio_ref?: string;
};

export type APISubscriberSummary = {
  imsi: string;
  profile_name: string;
  description?: string;
  msisdn?: string;
  status: SubscriberListStatus;
};

export type ListSubscribersResponse = {
  items: APISubscriberSummary[];
  page: number;
  per_page: number;
  total_count: number;
};

export type System = "5G" | "4G";

export type UEConnection = {
  amf_ue_ngap_id?: number;
  ran_ue_ngap_id?: number;
  mme_ue_s1ap_id?: number;
  enb_ue_s1ap_id?: number;
};

export type Registration = {
  system: System;
  registered: boolean;
  connection_state: ConnectionState | null;
  radio?: string;
  radio_ref?: string;
  last_seen_at?: string;
  imei?: string;
  ciphering_algorithm?: string;
  integrity_algorithm?: string;
  ims_voice_over_ps?: boolean;
  connection: UEConnection | null;
};

export type IMSUserState =
  | "registered"
  | "registered_unreg_services"
  | "authentication_pending"
  | "not_registered";

export type IMSPublicIdentity = {
  identity: string;
  barred: boolean;
  registration_set: "mmtel" | "mcptt";
  user_state: IMSUserState;
};

export type IMSSubscription = {
  private_identity: string;
  scscf_name?: string;
  public_identities: IMSPublicIdentity[];
};

export type SubscriberDetailStatus = {
  registered: boolean;
  connection_state?: ConnectionState;
  systems: System[];
  imei: string;
  ciphering_algorithm: string;
  integrity_algorithm: string;
  last_seen_at?: string;
  last_seen_radio?: string;
  last_seen_radio_ref?: string;
  ims_voice_over_ps?: boolean;
};

const isPresent = (registration: Registration) =>
  registration.connection_state !== null &&
  registration.connection_state !== undefined;

const seenAt = (registration?: Registration) =>
  registration?.last_seen_at ? Date.parse(registration.last_seen_at) : 0;

export function mergeRegistrations(
  registrations: Registration[],
): SubscriberDetailStatus {
  const present = registrations.filter(isPresent);

  const serving = present.reduce<Registration | undefined>(
    (best, candidate) =>
      !best || seenAt(candidate) > seenAt(best) ? candidate : best,
    undefined,
  );

  const retained = registrations.reduce<Registration | undefined>(
    (best, candidate) =>
      !best || seenAt(candidate) > seenAt(best) ? candidate : best,
    undefined,
  );

  const answering = serving ?? retained;

  const registered = serving?.registered
    ? serving
    : present.find((r) => r.registered);

  return {
    registered: present.some((r) => r.registered),
    connection_state:
      present.length === 0
        ? undefined
        : present.some((r) => r.connection_state === "connected")
          ? "connected"
          : "idle",
    systems: present.map((r) => r.system),
    imei: present.map((r) => r.imei ?? "").find(Boolean) ?? "",
    ciphering_algorithm: serving?.ciphering_algorithm ?? "",
    integrity_algorithm: serving?.integrity_algorithm ?? "",
    last_seen_at: answering?.last_seen_at,
    last_seen_radio: answering?.radio,
    last_seen_radio_ref: answering?.radio_ref,
    ims_voice_over_ps: registered?.ims_voice_over_ps,
  };
}

export type APISubscriber = {
  imsi: string;
  profile_name: string;
  description?: string;
  msisdn?: string;
  registrations: Registration[];
  sessions: SessionInfo[];
  ims?: IMSSubscription;
  ip_sm_gw?: IPSMGW;
};

export type IPSMGW = {
  number: string;
  diameter_host: string;
  diameter_realm: string;
};

export type DeregisterIMSResponse = {
  message: string;
  acknowledged: boolean;
};

export type SubscriberCredentials = {
  key: string;
  opc: string;
  sequenceNumber: string;
};

export async function listSubscribers(
  authToken: string,
  page: number,
  perPage: number,
  search?: string,
): Promise<ListSubscribersResponse> {
  const params = new URLSearchParams({
    page: String(page),
    per_page: String(perPage),
  });

  if (search) params.set("search", search);

  return apiFetch<ListSubscribersResponse>(
    `/api/v1/subscribers?${params.toString()}`,
    { authToken },
  );
}

export const SUBSCRIBER_SEARCH_LIMIT = 25;

export async function searchSubscriberImsis(
  authToken: string,
  search: string,
  dataNetwork?: string,
): Promise<string[]> {
  const params = new URLSearchParams({
    page: "1",
    per_page: String(SUBSCRIBER_SEARCH_LIMIT),
  });

  const trimmed = search.trim();
  if (trimmed) params.set("search", trimmed);
  if (dataNetwork) params.set("data_network", dataNetwork);

  const response = await apiFetch<ListSubscribersResponse>(
    `/api/v1/subscribers?${params.toString()}`,
    { authToken },
  );

  return (response.items ?? []).map((s) => s.imsi);
}

export async function listSubscribersByRadio(
  authToken: string,
  radioRef: string,
  page: number,
  perPage: number,
): Promise<ListSubscribersResponse> {
  return apiFetch<ListSubscribersResponse>(
    `/api/v1/subscribers?radio=${encodeURIComponent(radioRef)}&page=${page}&per_page=${perPage}`,
    { authToken },
  );
}

export const getSubscriber = async (
  authToken: string,
  imsi: string,
): Promise<APISubscriber> => {
  return apiFetch<APISubscriber>(`/api/v1/subscribers/${imsi}`, { authToken });
};

export const getSubscriberCredentials = async (
  authToken: string,
  imsi: string,
): Promise<SubscriberCredentials> => {
  return apiFetch<SubscriberCredentials>(
    `/api/v1/subscribers/${imsi}/credentials`,
    { authToken },
  );
};

export const createSubscriber = async (
  authToken: string,
  imsi: string,
  key: string,
  sequenceNumber: string,
  profileName: string,
  opc: string,
  description: string,
  msisdn: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/subscribers`, {
    method: "POST",
    authToken,
    body: {
      imsi,
      key,
      sequenceNumber,
      profile_name: profileName,
      opc,
      description,
      msisdn,
    },
  });
};

export const updateSubscriber = async (
  authToken: string,
  imsi: string,
  profileName: string,
  description: string,
  msisdn: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/subscribers/${imsi}`, {
    method: "PUT",
    authToken,
    body: { profile_name: profileName, description, msisdn },
  });
};

export const deregisterSubscriberIMS = async (
  authToken: string,
  imsi: string,
): Promise<DeregisterIMSResponse> => {
  return apiFetch<DeregisterIMSResponse>(
    `/api/v1/subscribers/${imsi}/ims/deregister`,
    { method: "POST", authToken },
  );
};

export const deleteSubscriber = async (
  authToken: string,
  name: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/subscribers/${name}`, {
    method: "DELETE",
    authToken,
  });
};

export interface SnssaiInfo {
  sst: number;
  sd?: string;
}

export interface SliceInfo {
  sst: number;
  sd?: string;
}

export interface SessionInfo {
  system: System;
  id: number;
  status: string;
  ip_type?: string; // IPv4 | IPv6 | IPv4v6
  ipv4_address?: string;
  ipv6_prefix?: string;
  data_network?: string;
  slice?: SliceInfo;
  ambr_uplink?: string;
  ambr_downlink?: string;
}
