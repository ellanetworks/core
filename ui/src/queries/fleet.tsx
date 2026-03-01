import { apiFetch } from "@/queries/utils";

export type RegisterFleetResponse = {
  message: string;
};

export type UnregisterFleetResponse = {
  message: string;
};

export type FleetURLResponse = {
  url: string;
};

export type UpdateFleetURLResponse = {
  message: string;
};

export async function getFleetURL(
  authToken: string,
): Promise<FleetURLResponse> {
  return apiFetch<FleetURLResponse>(`/api/v1/fleet/url`, {
    method: "GET",
    authToken,
  });
}

export async function updateFleetURL(
  authToken: string,
  url: string,
): Promise<UpdateFleetURLResponse> {
  return apiFetch<UpdateFleetURLResponse>(`/api/v1/fleet/url`, {
    method: "PUT",
    authToken,
    body: { url },
  });
}

export async function registerFleet(
  authToken: string,
  activationToken: string,
): Promise<RegisterFleetResponse> {
  return apiFetch<RegisterFleetResponse>(`/api/v1/fleet/register`, {
    method: "POST",
    authToken,
    body: { activationToken },
  });
}

export async function unregisterFleet(
  authToken: string,
): Promise<UnregisterFleetResponse> {
  return apiFetch<UnregisterFleetResponse>(`/api/v1/fleet/unregister`, {
    method: "POST",
    authToken,
  });
}
