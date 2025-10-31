import { HTTPStatus } from "@/queries/utils";

export type NetworkLogRetentionPolicy = {
  days: number;
};

export type APINetworkLog = {
  id: number;
  timestamp: string;
  protocol: string;
  message_type: string;
  direction: string;
  local_address: string;
  remote_address: string;
  details?: string;
};

export type ListNetworkLogsResponse = {
  items: APINetworkLog[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listNetworkLogs(
  authToken: string,
  page: number,
  perPage: number,
  params?: Record<string, string | string[]>,
): Promise<ListNetworkLogsResponse> {
  const url = new URL(`/api/v1/logs/network`, window.location.origin);
  url.searchParams.set("page", String(page));
  url.searchParams.set("per_page", String(perPage));

  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (Array.isArray(v)) url.searchParams.append(k, v.join(","));
      else url.searchParams.append(k, v);
    }
  }

  const response = await fetch(url.toString(), {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${authToken}`,
    },
  });

  let json: { result: ListNetworkLogsResponse; error?: string };
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

export type EnumField = {
  label: string;
  value: number;
  type: "enum";
};

export type DecodedNetworkLog = {
  pdu_type: string;
  message_type: string;
  procedure_code: EnumField;
  criticality: EnumField;
  value: unknown;
};

export type NetworkLogContent = {
  decoded: DecodedNetworkLog;
  raw: string;
};

export async function getNetworkLog(
  authToken: string,
  id: string,
): Promise<NetworkLogContent> {
  const url = new URL(`/api/v1/logs/network/${id}`, window.location.origin);

  const response = await fetch(url.toString(), {
    method: "GET",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${authToken}`,
    },
  });

  let json: { result: NetworkLogContent; error?: string };
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

export async function clearNetworkLogs(authToken: string): Promise<void> {
  const response = await fetch(`/api/v1/logs/network`, {
    method: "DELETE",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${authToken}`,
    },
  });

  if (!response.ok) {
    let respData;
    try {
      respData = await response.json();
    } catch {
      throw new Error(
        `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
      );
    }
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }
}

export const getNetworkLogRetentionPolicy = async (authToken: string) => {
  const response = await fetch(`/api/v1/logs/network/retention`, {
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

  return respData.result;
};

export const updateNetworkLogRetentionPolicy = async (
  authToken: string,
  days: number,
) => {
  const response = await fetch(`/api/v1/logs/network/retention`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + authToken,
    },
    body: JSON.stringify({ days: days }),
  });

  if (!response.ok) {
    let respData;
    try {
      respData = await response.json();
    } catch {
      throw new Error(
        `${response.status}: ${HTTPStatus(response.status)}. ${response.statusText}`,
      );
    }
    throw new Error(
      `${response.status}: ${HTTPStatus(response.status)}. ${respData?.error || "Unknown error"}`,
    );
  }
};
