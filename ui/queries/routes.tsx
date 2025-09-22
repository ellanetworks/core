import { HTTPStatus } from "@/queries/utils";
import { Route } from "@/types/types";

export const listRoutes = async (authToken: string): Promise<Route[]> => {
  const response = await fetch(`/api/v1/networking/routes`, {
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

  const transformed: Route[] = respData.result.map((p: any) => ({
    id: p.id,
    destination: p.destination,
    gateway: p.gateway,
    interface: p.interface,
    metric: p.metric,
  }));

  return transformed;
};

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
