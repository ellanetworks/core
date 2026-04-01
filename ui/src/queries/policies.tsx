import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type PolicyRule = {
  description: string;
  remote_prefix?: string;
  protocol: number;
  port_low: number;
  port_high: number;
  action: "allow" | "deny";
};

export type PolicyRules = {
  uplink?: PolicyRule[];
  downlink?: PolicyRule[];
};

export type APIPolicy = {
  name: string;
  bitrate_uplink: string;
  bitrate_downlink: string;
  var5qi: number;
  arp: number;
  data_network_name: string;
  rules?: PolicyRules;
};

export type ListPoliciesResponse = {
  items: APIPolicy[];
  page: number;
  per_page: number;
  total_count: number;
};

export const getPolicy = async (
  authToken: string,
  name: string,
): Promise<APIPolicy> => {
  return apiFetch<APIPolicy>(`/api/v1/policies/${name}`, { authToken });
};

export async function listPolicies(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListPoliciesResponse> {
  return apiFetch<ListPoliciesResponse>(
    `/api/v1/policies?page=${page}&per_page=${perPage}`,
    { authToken },
  );
}

export const createPolicy = async (
  authToken: string,
  name: string,
  bitrateUplink: string,
  bitrateDownlink: string,
  var5qi: number,
  arp: number,
  dataNetworkName: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/policies`, {
    method: "POST",
    authToken,
    body: {
      name,
      bitrate_uplink: bitrateUplink,
      bitrate_downlink: bitrateDownlink,
      var5qi,
      arp,
      data_network_name: dataNetworkName,
    },
  });
};

export const updatePolicy = async (
  authToken: string,
  name: string,
  bitrateUplink: string,
  bitrateDownlink: string,
  var5qi: number,
  arp: number,
  dataNetworkName: string,
  rules?: PolicyRules,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/policies/${name}`, {
    method: "PUT",
    authToken,
    body: {
      bitrate_uplink: bitrateUplink,
      bitrate_downlink: bitrateDownlink,
      var5qi,
      arp,
      data_network_name: dataNetworkName,
      ...(rules && { rules }),
    },
  });
};

export const deletePolicy = async (
  authToken: string,
  name: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/policies/${name}`, {
    method: "DELETE",
    authToken,
  });
};
