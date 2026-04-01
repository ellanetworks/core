import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type DataNetworkStatus = {
  sessions: number;
};

export type DataNetworkIPAllocation = {
  pool_size: number;
  allocated: number;
  available: number;
};

export type APIDataNetwork = {
  name: string;
  ip_pool: string;
  dns: string;
  mtu: number;
  status?: DataNetworkStatus;
  ip_allocation?: DataNetworkIPAllocation;
};

export type APIIPAllocation = {
  address: string;
  imsi: string;
  type: string;
  session_id: number | null;
};

export type ListIPAllocationsResponse = {
  items: APIIPAllocation[];
  page: number;
  per_page: number;
  total_count: number;
};

export type ListDataNetworksResponse = {
  items: APIDataNetwork[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listDataNetworks(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListDataNetworksResponse> {
  return apiFetch<ListDataNetworksResponse>(
    `/api/v1/networking/data-networks?page=${page}&per_page=${perPage}`,
    { authToken },
  );
}

export async function getDataNetwork(
  authToken: string,
  name: string,
): Promise<APIDataNetwork> {
  return apiFetch<APIDataNetwork>(
    `/api/v1/networking/data-networks/${encodeURIComponent(name)}`,
    { authToken },
  );
}

export async function listIPAllocations(
  authToken: string,
  name: string,
  page: number,
  perPage: number,
): Promise<ListIPAllocationsResponse> {
  return apiFetch<ListIPAllocationsResponse>(
    `/api/v1/networking/data-networks/${encodeURIComponent(name)}/ip-allocations?page=${page}&per_page=${perPage}`,
    { authToken },
  );
}

export const createDataNetwork = async (
  authToken: string,
  name: string,
  ipPool: string,
  dns: string,
  mtu: number,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/networking/data-networks`, {
    method: "POST",
    authToken,
    body: { name, ip_pool: ipPool, dns, mtu },
  });
};

export const updateDataNetwork = async (
  authToken: string,
  name: string,
  ipPool: string,
  dns: string,
  mtu: number,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/networking/data-networks/${name}`, {
    method: "PUT",
    authToken,
    body: { name, ip_pool: ipPool, dns, mtu },
  });
};

export const deleteDataNetwork = async (
  authToken: string,
  name: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/networking/data-networks/${name}`, {
    method: "DELETE",
    authToken,
  });
};
