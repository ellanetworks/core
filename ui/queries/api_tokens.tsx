import { apiFetch } from "@/queries/utils";

export type APIToken = {
  id: number;
  name: string;
  expires_at: string | null;
};

export type ListAPITokensResponse = {
  items: APIToken[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listAPITokens(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListAPITokensResponse> {
  return apiFetch<ListAPITokensResponse>(
    `/api/v1/users/me/api-tokens?page=${page}&per_page=${perPage}`,
    { authToken },
  );
}

export const createAPIToken = async (
  authToken: string,
  name: string,
  expires_at: string,
): Promise<{ token: string }> => {
  return apiFetch<{ token: string }>(`/api/v1/users/me/api-tokens`, {
    method: "POST",
    authToken,
    body: { name, expires_at },
  });
};

export const deleteAPIToken = async (authToken: string, id: number) => {
  return apiFetch(`/api/v1/users/me/api-tokens/${id}`, {
    method: "DELETE",
    authToken,
  });
};
