import { apiFetch, apiFetchVoid } from "@/queries/utils";

/** Lightweight status returned by the list endpoint. */
export type SubscriberListStatus = {
  registered?: boolean;
  num_pdu_sessions?: number;
  lastSeenAt?: string;
};

/** Summary representation returned by the list endpoint. */
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

/** Rich status returned by the get-single endpoint. */
export type SubscriberDetailStatus = {
  registered?: boolean;
  imei?: string;
  cipheringAlgorithm?: string;
  integrityAlgorithm?: string;
  lastSeenAt?: string;
  lastSeenRadio?: string;
};

/** Full representation returned by the get-single endpoint. */
export type APISubscriber = {
  imsi: string;
  profile_name: string;
  status: SubscriberDetailStatus;
  pdu_sessions: SessionInfo[];
};

/** Credentials returned by the dedicated credentials endpoint. */
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

export interface SessionInfo {
  pdu_session_id: number;
  status: string;
  ipAddress?: string;
}
