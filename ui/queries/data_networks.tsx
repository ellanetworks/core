import { HTTPStatus } from "@/queries/utils";
import { DataNetwork } from "@/types/types";

export const listDataNetworks = async (
  authToken: string,
): Promise<DataNetwork[]> => {
  const response = await fetch(`/api/v1/data-networks`, {
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

  const transformed: DataNetwork[] = respData.result.map((p: any) => ({
    name: p.name,
    ipPool: p["ip-pool"],
    dns: p.dns,
    mtu: p.mtu,
    status: p.status ? { sessions: p.status.sessions } : undefined,
  }));

  return transformed;
};

export const createDataNetwork = async (
  authToken: string,
  name: string,
  ipPool: string,
  dns: string,
  mtu: number,
) => {
  const policyData = {
    name: name,
    "ip-pool": ipPool,
    dns: dns,
    mtu: mtu,
  };

  const response = await fetch(`/api/v1/data-networks`, {
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

export const updateDataNetwork = async (
  authToken: string,
  name: string,
  ipPool: string,
  dns: string,
  mtu: number,
) => {
  const policyData = {
    name: name,
    "ip-pool": ipPool,
    dns: dns,
    mtu: mtu,
  };

  const response = await fetch(`/api/v1/data-networks/${name}`, {
    method: "PUT",
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

export const deleteDataNetwork = async (authToken: string, name: string) => {
  const response = await fetch(`/api/v1/data-networks/${name}`, {
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
