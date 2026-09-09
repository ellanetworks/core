// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

// Mirrors MaxDescriptionLength in internal/api/server/api_subscribers.go.
export const MAX_DESCRIPTION_LENGTH = 64;

export type ConnectionState = "idle" | "connected";

export type SubscriberListStatus = {
  registered?: boolean;
  connection_state?: ConnectionState;
  radio_access_types?: string[];
  num_sessions?: number;
  last_seen_at?: string;
  last_seen_radio?: string;
};

export type APISubscriberSummary = {
  imsi: string;
  profile_name: string;
  description?: string;
  status: SubscriberListStatus;
};

export type ListSubscribersResponse = {
  items: APISubscriberSummary[];
  page: number;
  per_page: number;
  total_count: number;
};

export type System = "5GS" | "EPS";

export type AccessType = "3GPP" | "non-3GPP";

export type UEConnection = {
  amf_ue_ngap_id?: number;
  ran_ue_ngap_id?: number;
  mme_ue_s1ap_id?: number;
  enb_ue_s1ap_id?: number;
};

export type Registration = {
  system: System;
  access_type: AccessType;
  registered: boolean;
  connection_state: ConnectionState | null;
  radio?: string;
  last_seen_at?: string;
  pei?: string;
  imei?: string;
  ciphering_algorithm?: string;
  integrity_algorithm?: string;
  connection: UEConnection | null;
};

export type SubscriberDetailStatus = {
  registered: boolean;
  connection_state?: ConnectionState;
  radio_access_types: string[];
  imei: string;
  ciphering_algorithm: string;
  integrity_algorithm: string;
  last_seen_at?: string;
  last_seen_radio?: string;
};

export const SYSTEM_ACCESS_LABELS: Record<System, string> = {
  "5GS": "5G",
  EPS: "4G",
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

  return {
    registered: present.some((r) => r.registered),
    connection_state:
      present.length === 0
        ? undefined
        : present.some((r) => r.connection_state === "connected")
          ? "connected"
          : "idle",
    radio_access_types: present.map(
      (r) => SYSTEM_ACCESS_LABELS[r.system] ?? r.system,
    ),
    imei: present.map((r) => r.imei ?? r.pei ?? "").find(Boolean) ?? "",
    ciphering_algorithm: serving?.ciphering_algorithm ?? "",
    integrity_algorithm: serving?.integrity_algorithm ?? "",
    last_seen_at: answering?.last_seen_at,
    last_seen_radio: answering?.radio,
  };
}

export type APISubscriber = {
  imsi: string;
  profile_name: string;
  description?: string;
  registrations: Registration[];
  sessions: SessionInfo[];
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

// The API caps per_page at 100 (internal/api/server/api_subscribers.go), so the
// roster is assembled here. Fetching it whole is only reasonable because
// MaxNumSubscribers is 1000; a materially higher cap wants the search parameter
// and an async autocomplete instead.
const ROSTER_PER_PAGE = 100;

/**
 * Every subscriber's IMSI, for filters that must offer subscribers a filtered
 * query cannot see — one whose traffic is entirely dropped has flow reports and
 * no usage row, and one idle in the selected range has neither.
 */
export async function listAllSubscriberImsis(
  authToken: string,
): Promise<string[]> {
  const first = await listSubscribers(authToken, 1, ROSTER_PER_PAGE);
  const items = first.items ?? [];
  const totalCount = first.total_count ?? items.length;

  const pageCount = Math.ceil(totalCount / ROSTER_PER_PAGE);
  const rest =
    pageCount > 1
      ? await Promise.all(
          Array.from({ length: pageCount - 1 }, (_, i) =>
            listSubscribers(authToken, i + 2, ROSTER_PER_PAGE),
          ),
        )
      : [];

  return items
    .concat(...rest.map((r) => r.items ?? []))
    .map((s) => s.imsi)
    .sort((a, b) => a.localeCompare(b));
}

export async function listSubscribersByRadio(
  authToken: string,
  radioName: string,
  page: number,
  perPage: number,
): Promise<ListSubscribersResponse> {
  return apiFetch<ListSubscribersResponse>(
    `/api/v1/subscribers?radio=${encodeURIComponent(radioName)}&page=${page}&per_page=${perPage}`,
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
    },
  });
};

export const updateSubscriber = async (
  authToken: string,
  imsi: string,
  profileName: string,
  description: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/subscribers/${imsi}`, {
    method: "PUT",
    authToken,
    body: { profile_name: profileName, description },
  });
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
  access_types: AccessType[];
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
