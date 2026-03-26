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

export type BGPImportPrefix = {
  prefix: string;
  maxLength: number;
};

export type BGPPeer = {
  id: number;
  address: string;
  remoteAS: number;
  holdTime: number;
  password: string;
  description: string;
  importPrefixes: BGPImportPrefix[];
  state?: string;
  uptime?: string;
  prefixesSent?: number;
  prefixesReceived?: number;
  prefixesAccepted?: number;
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
  importPrefixes?: BGPImportPrefix[];
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

export type UpdateBGPPeerParams = {
  address: string;
  remoteAS: number;
  holdTime: number;
  password?: string;
  description?: string;
  importPrefixes?: BGPImportPrefix[];
};

export async function updateBGPPeer(
  authToken: string,
  id: number,
  params: UpdateBGPPeerParams,
): Promise<BGPPeer> {
  return apiFetch<BGPPeer>(`/api/v1/networking/bgp/peers/${id}`, {
    method: "PUT",
    authToken,
    body: params,
  });
}

export async function getBGPPeer(
  authToken: string,
  id: number,
): Promise<BGPPeer> {
  return apiFetch<BGPPeer>(`/api/v1/networking/bgp/peers/${id}`, {
    authToken,
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

// BGP Advertised Routes

export type BGPAdvertisedRoute = {
  subscriber: string;
  prefix: string;
  nextHop: string;
};

export type BGPAdvertisedRoutesResponse = {
  routes: BGPAdvertisedRoute[];
};

export async function getBGPAdvertisedRoutes(
  authToken: string,
): Promise<BGPAdvertisedRoutesResponse> {
  return apiFetch<BGPAdvertisedRoutesResponse>(
    "/api/v1/networking/bgp/advertised-routes",
    { authToken },
  );
}

// BGP Learned Routes

export type BGPLearnedRoute = {
  prefix: string;
  nextHop: string;
  peer: string;
};

export type BGPLearnedRoutesResponse = {
  routes: BGPLearnedRoute[];
};

export async function getBGPLearnedRoutes(
  authToken: string,
): Promise<BGPLearnedRoutesResponse> {
  return apiFetch<BGPLearnedRoutesResponse>(
    "/api/v1/networking/bgp/learned-routes",
    { authToken },
  );
}
