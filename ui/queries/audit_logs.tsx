import { HTTPStatus } from "@/queries/utils";

export type AuditLogRetentionPolicy = {
  days: number;
};

export type APIAuditLog = {
  id: number;
  timestamp: string;
  level: string;
  actor: string;
  action: string;
  ip: string;
  details: string;
};

export type ListAuditLogsResponse = {
  items: APIAuditLog[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listAuditLogs(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListAuditLogsResponse> {
  const response = await fetch(
    `/api/v1/logs/audit?page=${page}&per_page=${perPage}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + authToken,
      },
    },
  );
  let json: { result: ListAuditLogsResponse; error?: string };
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

export const getAuditLogRetentionPolicy = async (authToken: string) => {
  const response = await fetch(`/api/v1/logs/audit/retention`, {
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

export const updateAuditLogRetentionPolicy = async (
  authToken: string,
  days: number,
) => {
  const response = await fetch(`/api/v1/logs/audit/retention`, {
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
