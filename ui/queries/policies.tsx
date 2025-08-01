import { HTTPStatus } from "@/queries/utils";
import { Policy } from "@/types/types";

export const listPolicies = async (authToken: string): Promise<Policy[]> => {
  const response = await fetch(`/api/v1/policies`, {
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

  const transformed: Policy[] = respData.result.map((p: any) => ({
    name: p.name,
    bitrateUp: p["bitrate-uplink"],
    bitrateDown: p["bitrate-downlink"],
    fiveQi: p["var5qi"],
    priorityLevel: p["priority-level"],
    dataNetworkName: p["data-network-name"],
  }));

  return transformed;
};

export const createPolicy = async (
  authToken: string,
  name: string,
  bitrateUplink: string,
  bitrateDownlink: string,
  var5qi: number,
  priorityLevel: number,
  dataNetworkName: string,
) => {
  const policyData = {
    name: name,
    "bitrate-uplink": bitrateUplink,
    "bitrate-downlink": bitrateDownlink,
    var5qi: var5qi,
    "priority-level": priorityLevel,
    "data-network-name": dataNetworkName,
  };

  const response = await fetch(`/api/v1/policies`, {
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

export const updatePolicy = async (
  authToken: string,
  name: string,
  bitrateUplink: string,
  bitrateDownlink: string,
  var5qi: number,
  priorityLevel: number,
  dataNetworkName: string,
) => {
  const policyData = {
    name: name,
    "bitrate-uplink": bitrateUplink,
    "bitrate-downlink": bitrateDownlink,
    var5qi: var5qi,
    "priority-level": priorityLevel,
    "data-network-name": dataNetworkName,
  };

  const response = await fetch(`/api/v1/policies/${name}`, {
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

export const deletePolicy = async (authToken: string, name: string) => {
  const response = await fetch(`/api/v1/policies/${name}`, {
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
