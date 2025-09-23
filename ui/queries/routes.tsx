import { HTTPStatus } from "@/queries/utils";

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
  const response = await fetch(
    `/api/v1/networking/routes?page=${page}&per_page=${perPage}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + authToken,
      },
    },
  );
  let json: { result: ListRoutesResponse; error?: string };
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

export const createRoute = async (
  authToken: string,
  destination: string,
  gateway: string,
  interfaceName: string,
  metric: number,
) => {
  const routeData = {
    destination: destination,
    gateway: gateway,
    interface: interfaceName,
    metric: metric,
  };

  const response = await fetch(`/api/v1/networking/routes`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
    body: JSON.stringify(routeData),
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

export const deleteRoute = async (authToken: string, id: number) => {
  const response = await fetch(`/api/v1/networking/routes/${id}`, {
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
