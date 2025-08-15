import { HTTPStatus } from "@/queries/utils";
import { APIToken } from "@/types/types";

export const listAPITokens = async (authToken: string): Promise<APIToken[]> => {
  const response = await fetch(`/api/v1/api-tokens`, {
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
    expiresAt: p.expiresAt,
  }));

  return transformed;
};

export const createAPIToken = async (
  authToken: string,
  name: string,
  expiry: string,
) => {
  const policyData = {
    name: name,
    expiry: expiry,
  };

  const response = await fetch(`/api/v1/api-tokens`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
    body: JSON.stringify(policyData),
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
  const response = await fetch(`/api/v1/api-tokens/${id}`, {
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
