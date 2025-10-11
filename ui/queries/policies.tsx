import { HTTPStatus } from "@/queries/utils";

export type APIPolicy = {
  name: string;
  bitrate_uplink: string;
  bitrate_downlink: string;
  var5qi: number;
  arp: number;
  data_network_name: string;
};

export type ListPoliciesResponse = {
  items: APIPolicy[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listPolicies(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListPoliciesResponse> {
  const response = await fetch(
    `/api/v1/policies?page=${page}&per_page=${perPage}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + authToken,
      },
    },
  );

  let json: { result: ListPoliciesResponse; error?: string };
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

export const createPolicy = async (
  authToken: string,
  name: string,
  bitrateUplink: string,
  bitrateDownlink: string,
  var5qi: number,
  arp: number,
  dataNetworkName: string,
) => {
  const policyData = {
    name: name,
    bitrate_uplink: bitrateUplink,
    bitrate_downlink: bitrateDownlink,
    var5qi: var5qi,
    arp: arp,
    data_network_name: dataNetworkName,
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
  arp: number,
  dataNetworkName: string,
) => {
  const policyData = {
    name: name,
    bitrate_uplink: bitrateUplink,
    bitrate_downlink: bitrateDownlink,
    var5qi: var5qi,
    arp: arp,
    data_network_name: dataNetworkName,
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
