import { HTTPStatus } from "@/queries/utils";

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
  const response = await fetch(
    `/api/v1/users/me/api-tokens?page=${page}&per_page=${perPage}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + authToken,
      },
    },
  );

  let json: { result: ListAPITokensResponse; error?: string };
  try {
    json = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${json?.error || "Unknown error"}`,
    );
  }

  return json.result;
}

export const createAPIToken = async (
  authToken: string,
  name: string,
  expires_at: string,
) => {
  const data = {
    name: name,
    expires_at: expires_at,
  };

  const response = await fetch(`/api/v1/users/me/api-tokens`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
    body: JSON.stringify(data),
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};

export const deleteAPIToken = async (authToken: string, id: number) => {
  const response = await fetch(`/api/v1/users/me/api-tokens/${id}`, {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
  });
  let respData;
  try {
    respData = await response.json();
  } catch {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
    );
  }

  if (!response.ok) {
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }

  return respData.result;
};
