import { apiFetch, apiFetchVoid } from "@/queries/utils";

// BGP Settings

export type BGPSettings = {
  enabled: boolean;
  localAS: number;
  routerID: string;
  listenAddress: string;
};

export type UpdateBGPSettingsParams = {
  enabled: boolean;
  localAS: number;
  routerID: string;
  listenAddress: string;
};

export async function getBGPSettings(authToken: string): Promise<BGPSettings> {
  return apiFetch<BGPSettings>("/api/v1/networking/bgp", { authToken });
}

export async function updateBGPSettings(
  authToken: string,
  params: UpdateBGPSettingsParams,
): Promise<void> {
  await apiFetchVoid("/api/v1/networking/bgp", {
    method: "PUT",
    authToken,
    body: params,
  });
}

// BGP Peers

export type BGPPeer = {
  id: number;
  address: string;
  remoteAS: number;
  holdTime: number;
  password: string;
  description: string;
  state?: string;
  uptime?: string;
  prefixesSent?: number;
};

export type ListBGPPeersResponse = {
  items: BGPPeer[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listBGPPeers(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListBGPPeersResponse> {
  return apiFetch<ListBGPPeersResponse>(
    `/api/v1/networking/bgp/peers?page=${page}&per_page=${perPage}`,
    { authToken },
  );
}

export type CreateBGPPeerParams = {
  address: string;
  remoteAS: number;
  holdTime: number;
  password?: string;
  description?: string;
};

export async function createBGPPeer(
  authToken: string,
  params: CreateBGPPeerParams,
): Promise<void> {
  await apiFetchVoid("/api/v1/networking/bgp/peers", {
    method: "POST",
    authToken,
    body: params,
  });
}

export async function deleteBGPPeer(
  authToken: string,
  id: number,
): Promise<void> {
  await apiFetchVoid(`/api/v1/networking/bgp/peers/${id}`, {
    method: "DELETE",
    authToken,
  });
}

// BGP Routes

export type BGPRoute = {
  subscriber: string;
  prefix: string;
  nextHop: string;
};

export type BGPRoutesResponse = {
  routes: BGPRoute[];
};

export async function getBGPRoutes(
  authToken: string,
): Promise<BGPRoutesResponse> {
  return apiFetch<BGPRoutesResponse>("/api/v1/networking/bgp/routes", {
    authToken,
  });
}
