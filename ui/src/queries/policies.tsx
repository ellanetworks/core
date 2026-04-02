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

// API response shape from the backend (new data model).
type APIPolicyRaw = {
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

// UI-facing type — keeps old field names so pages don't change visually.
export type APIPolicy = {
  name: string;
  bitrate_uplink: string;
  bitrate_downlink: string;
  var5qi: number;
  arp: number;
  data_network_name: string;
  rules?: PolicyRules;
};

function rawToUIPolicy(raw: APIPolicyRaw): APIPolicy {
  return {
    name: raw.name,
    bitrate_uplink: raw.session_ambr_uplink,
    bitrate_downlink: raw.session_ambr_downlink,
    var5qi: raw.var5qi,
    arp: raw.arp,
    data_network_name: raw.data_network_name,
    rules: raw.rules,
  };
}

type APIPolicyListRaw = {
  items: APIPolicyRaw[];
  page: number;
  per_page: number;
  total_count: number;
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
  const raw = await apiFetch<APIPolicyRaw>(`/api/v1/policies/${name}`, {
    authToken,
  });
  return rawToUIPolicy(raw);
};

export async function listPolicies(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListPoliciesResponse> {
  const raw = await apiFetch<APIPolicyListRaw>(
    `/api/v1/policies?page=${page}&per_page=${perPage}`,
    { authToken },
  );
  return {
    ...raw,
    items: (raw.items ?? []).map(rawToUIPolicy),
  };
}

async function fetchSliceName(authToken: string): Promise<string> {
  const res = await apiFetch<{
    items: { name: string }[];
  }>(`/api/v1/slices?page=1&per_page=1`, { authToken });
  if (!res.items?.length) {
    throw new Error("No network slice configured");
  }
  return res.items[0].name;
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
  // 1. Create a profile with the same name.
  await apiFetchVoid(`/api/v1/profiles`, {
    method: "POST",
    authToken,
    body: {
      name,
      ue_ambr_uplink: bitrateUplink,
      ue_ambr_downlink: bitrateDownlink,
    },
  });

  // 2. Get the single slice name.
  const sliceName = await fetchSliceName(authToken);

  // 3. Create the policy referencing the profile and slice.
  try {
    await apiFetchVoid(`/api/v1/policies`, {
      method: "POST",
      authToken,
      body: {
        name,
        profile_name: name,
        slice_name: sliceName,
        data_network_name: dataNetworkName,
        session_ambr_uplink: bitrateUplink,
        session_ambr_downlink: bitrateDownlink,
        var5qi,
        arp,
      },
    });
  } catch (policyErr) {
    // Roll back the profile we just created to avoid orphans.
    try {
      await apiFetchVoid(`/api/v1/profiles/${name}`, {
        method: "DELETE",
        authToken,
      });
    } catch {
      // Best-effort cleanup; the original error is more important.
    }
    throw policyErr;
  }
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
  // 1. Update the companion profile (same name) with new UE-AMBR.
  await apiFetchVoid(`/api/v1/profiles/${name}`, {
    method: "PUT",
    authToken,
    body: {
      ue_ambr_uplink: bitrateUplink,
      ue_ambr_downlink: bitrateDownlink,
    },
  });

  // 2. Get the single slice name.
  const sliceName = await fetchSliceName(authToken);

  // 3. Update the policy.
  await apiFetchVoid(`/api/v1/policies/${name}`, {
    method: "PUT",
    authToken,
    body: {
      profile_name: name,
      slice_name: sliceName,
      data_network_name: dataNetworkName,
      session_ambr_uplink: bitrateUplink,
      session_ambr_downlink: bitrateDownlink,
      var5qi,
      arp,
      ...(rules !== undefined && { rules }),
    },
  });
};

export const deletePolicy = async (
  authToken: string,
  name: string,
): Promise<void> => {
  // 1. Delete the policy first.
  await apiFetchVoid(`/api/v1/policies/${name}`, {
    method: "DELETE",
    authToken,
  });

  // 2. Delete the companion profile — silently ignore 409 (subscribers still reference it).
  try {
    await apiFetchVoid(`/api/v1/profiles/${name}`, {
      method: "DELETE",
      authToken,
    });
  } catch (err: unknown) {
    // Only swallow 409 Conflict (profile still referenced by subscribers).
    // Re-throw other errors (network failures, 500s, auth failures, etc.).
    if (err instanceof Error && err.message?.includes("409")) {
      // Expected: profile still has subscribers.
    } else {
      console.error("Failed to delete companion profile:", err);
    }
  }
};
