import { apiFetch } from "@/queries/utils";

export type SubscriberStatus = {
  registered?: boolean;
  ipAddress?: string;
};

export type APISubscriber = {
  imsi: string;
  opc: string;
  sequenceNumber: string;
  key: string;
  policyName: string;
  status: SubscriberStatus;
};

export type ListSubscribersResponse = {
  items: APISubscriber[];
  page: number;
  per_page: number;
  total_count: number;
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

export const createSubscriber = async (
  authToken: string,
  imsi: string,
  key: string,
  sequenceNumber: string,
  policyName: string,
  opc: string,
) => {
  return apiFetch(`/api/v1/subscribers`, {
    method: "POST",
    authToken,
    body: { imsi, key, sequenceNumber, policyName, opc },
  });
};

export const updateSubscriber = async (
  authToken: string,
  imsi: string,
  policyName: string,
) => {
  return apiFetch(`/api/v1/subscribers/${imsi}`, {
    method: "PUT",
    authToken,
    body: { imsi, policyName },
  });
};

export const deleteSubscriber = async (authToken: string, name: string) => {
  return apiFetch(`/api/v1/subscribers/${name}`, {
    method: "DELETE",
    authToken,
  });
};
