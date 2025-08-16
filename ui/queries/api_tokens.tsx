import { HTTPStatus } from "@/queries/utils";
import { APIToken } from "@/types/types";

export const listAPITokens = async (authToken: string): Promise<APIToken[]> => {
  const response = await fetch(`/api/v1/users/me/api-tokens`, {
    method: "GET",
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

  const transformed: APIToken[] = respData.result.map((p: any) => ({
    id: p.id,
    name: p.name,
    expires_at: p.expires_at,
  }));

  return transformed;
};

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
