import { apiFetch, apiFetchVoid } from "@/queries/utils";

/** Lightweight status returned by the list endpoint. */
export type SubscriberListStatus = {
  registered?: boolean;
  ipAddress?: string;
};

/** Summary representation returned by the list endpoint. */
export type APISubscriberSummary = {
  imsi: string;
  policyName: string;
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
  ipAddress?: string;
  imei?: string;
  cipheringAlgorithm?: string;
  integrityAlgorithm?: string;
  lastSeenAt?: string;
  lastSeenRadio?: string;
};

/** Full representation returned by the get-single endpoint. */
export type APISubscriber = {
  imsi: string;
  policyName: string;
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
  policyName: string,
  opc: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/subscribers`, {
    method: "POST",
    authToken,
    body: { imsi, key, sequenceNumber, policyName, opc },
  });
};

export const updateSubscriber = async (
  authToken: string,
  imsi: string,
  policyName: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/subscribers/${imsi}`, {
    method: "PUT",
    authToken,
    body: { imsi, policyName },
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
  status: string;
  ipAddress?: string;
}
