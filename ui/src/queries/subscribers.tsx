// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type SubscriberListStatus = {
  registered?: boolean;
  radio_access_type?: string; // "4G" | "5G", per the live connection
  num_sessions?: number;
  last_seen_at?: string;
};

export type APISubscriberSummary = {
  imsi: string;
  profile_name: string;
  radio?: string;
  status: SubscriberListStatus;
};

export type ListSubscribersResponse = {
  items: APISubscriberSummary[];
  page: number;
  per_page: number;
  total_count: number;
};

export type SubscriberDetailStatus = {
  registered?: boolean;
  radio_access_type?: string; // "4G" | "5G", per the live connection
  imei?: string;
  ciphering_algorithm?: string;
  integrity_algorithm?: string;
  last_seen_at?: string;
  last_seen_radio?: string;
};

export type APISubscriber = {
  imsi: string;
  profile_name: string;
  status: SubscriberDetailStatus;
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
): Promise<ListSubscribersResponse> {
  return apiFetch<ListSubscribersResponse>(
    `/api/v1/subscribers?page=${page}&per_page=${perPage}`,
    { authToken },
  );
}

// The API caps per_page at 100 (internal/api/server/api_subscribers.go), so the
// roster is assembled here. Fetching it whole is only reasonable because
// MaxNumSubscribers is 1000; a materially higher cap needs a search parameter
// instead.
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
): Promise<void> => {
  await apiFetchVoid(`/api/v1/subscribers`, {
    method: "POST",
    authToken,
    body: { imsi, key, sequenceNumber, profile_name: profileName, opc },
  });
};

export const updateSubscriber = async (
  authToken: string,
  imsi: string,
  profileName: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/subscribers/${imsi}`, {
    method: "PUT",
    authToken,
    body: { profile_name: profileName },
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
  radio_access_type: string; // "4G" | "5G"
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
