import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type NetworkRule = {
  id: number;
  policy_id: number;
  description: string;
  direction: "uplink" | "downlink";
  remote_prefix?: string;
  protocol: number;
  port_low: number;
  port_high: number;
  action: "allow" | "deny";
  precedence: number;
  created_at: string;
  updated_at: string;
};

export type ListNetworkRulesResponse = {
  items: NetworkRule[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listNetworkRules(
  authToken: string,
  policyName: string,
): Promise<ListNetworkRulesResponse> {
  return apiFetch<ListNetworkRulesResponse>(
    `/api/v1/policies/${encodeURIComponent(policyName)}/rules`,
    { authToken },
  );
}

export async function reorderNetworkRule(
  authToken: string,
  policyName: string,
  ruleId: number,
  newIndex: number,
): Promise<void> {
  await apiFetchVoid(
    `/api/v1/policies/${encodeURIComponent(policyName)}/rules/${ruleId}/reorder`,
    {
      method: "POST",
      authToken,
      body: {
        new_index: newIndex,
      },
    },
  );
}

export async function createNetworkRule(
  authToken: string,
  policyName: string,
  description: string,
  direction: "uplink" | "downlink",
  action: "allow" | "deny",
  precedence: number,
  remotePrefix?: string,
  protocol?: number,
  portLow?: number,
  portHigh?: number,
): Promise<{ id: number }> {
  return apiFetch<{ id: number }>(
    `/api/v1/policies/${encodeURIComponent(policyName)}/rules`,
    {
      method: "POST",
      authToken,
      body: {
        description,
        direction,
        action,
        precedence,
        remote_prefix: remotePrefix,
        protocol,
        port_low: portLow,
        port_high: portHigh,
      },
    },
  );
}

export async function updateNetworkRule(
  authToken: string,
  policyName: string,
  ruleId: number,
  direction: "uplink" | "downlink",
  action: "allow" | "deny",
  precedence: number,
  remotePrefix?: string,
  protocol?: number,
  portLow?: number,
  portHigh?: number,
): Promise<void> {
  await apiFetchVoid(
    `/api/v1/policies/${encodeURIComponent(policyName)}/rules/${ruleId}`,
    {
      method: "PUT",
      authToken,
      body: {
        direction,
        action,
        precedence,
        remote_prefix: remotePrefix,
        protocol,
        port_low: portLow,
        port_high: portHigh,
      },
    },
  );
}

export async function deleteNetworkRule(
  authToken: string,
  policyName: string,
  ruleId: number,
): Promise<void> {
  await apiFetchVoid(
    `/api/v1/policies/${encodeURIComponent(policyName)}/rules/${ruleId}`,
    {
      method: "DELETE",
      authToken,
    },
  );
}

export async function getNetworkRule(
  authToken: string,
  policyName: string,
  ruleId: number,
): Promise<NetworkRule> {
  return apiFetch<NetworkRule>(
    `/api/v1/policies/${encodeURIComponent(policyName)}/rules/${ruleId}`,
    {
      authToken,
    },
  );
}
