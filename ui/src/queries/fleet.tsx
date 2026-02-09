import { apiFetch } from "@/queries/utils";

export type RegisterFleetResponse = {
  message: string;
};

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
