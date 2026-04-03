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
  profile_name: string;
  slice_name: string;
  data_network_name: string;
  session_ambr_uplink: string;
  session_ambr_downlink: string;
  var5qi: number;
  arp: number;
  rules?: PolicyRules;
};

export type ListPoliciesResponse = {
  items: APIPolicy[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function getPolicy(
  authToken: string,
  name: string,
): Promise<APIPolicy> {
  return apiFetch<APIPolicy>(`/api/v1/policies/${encodeURIComponent(name)}`, {
    authToken,
  });
}

export async function listPolicies(
  authToken: string,
  page: number,
  perPage: number,
  profileName?: string,
): Promise<ListPoliciesResponse> {
  let url = `/api/v1/policies?page=${page}&per_page=${perPage}`;
  if (profileName) {
    url += `&profile_name=${encodeURIComponent(profileName)}`;
  }
  return apiFetch<ListPoliciesResponse>(url, { authToken });
}

export async function createPolicy(
  authToken: string,
  body: {
    name: string;
    profile_name: string;
    slice_name: string;
    data_network_name: string;
    session_ambr_uplink: string;
    session_ambr_downlink: string;
    var5qi: number;
    arp: number;
  },
): Promise<void> {
  await apiFetchVoid(`/api/v1/policies`, {
    method: "POST",
    authToken,
    body,
  });
}

export async function updatePolicy(
  authToken: string,
  name: string,
  body: {
    profile_name: string;
    slice_name: string;
    data_network_name: string;
    session_ambr_uplink: string;
    session_ambr_downlink: string;
    var5qi: number;
    arp: number;
    rules?: PolicyRules;
  },
): Promise<void> {
  await apiFetchVoid(`/api/v1/policies/${encodeURIComponent(name)}`, {
    method: "PUT",
    authToken,
    body,
  });
}

export async function deletePolicy(
  authToken: string,
  name: string,
): Promise<void> {
  await apiFetchVoid(`/api/v1/policies/${encodeURIComponent(name)}`, {
    method: "DELETE",
    authToken,
  });
}
