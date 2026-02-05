import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type APIRoute = {
  id: string;
  destination: string;
  gateway: string;
  interface: string;
  metric: number;
};

export type ListRoutesResponse = {
  items: APIRoute[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listRoutes(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListRoutesResponse> {
  return apiFetch<ListRoutesResponse>(
    `/api/v1/networking/routes?page=${page}&per_page=${perPage}`,
    { authToken },
  );
}

export const createRoute = async (
  authToken: string,
  destination: string,
  gateway: string,
  interfaceName: string,
  metric: number,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/networking/routes`, {
    method: "POST",
    authToken,
    body: { destination, gateway, interface: interfaceName, metric },
  });
};

export const deleteRoute = async (
  authToken: string,
  id: number,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/networking/routes/${id}`, {
    method: "DELETE",
    authToken,
  });
};
