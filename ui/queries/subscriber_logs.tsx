import { HTTPStatus } from "@/queries/utils";

export type SubscriberLogRetentionPolicy = {
  days: number;
};

export type APISubscriberLog = {
  id: number;
  timestamp: string;
  imsi: string;
  event: string;
  direction: string;
  details?: string;
};

export type ListSubscriberLogsResponse = {
  items: APISubscriberLog[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listSubscriberLogs(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListSubscriberLogsResponse> {
  const response = await fetch(
    `/api/v1/logs/subscriber?page=${page}&per_page=${perPage}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${authToken}`,
      },
    },
  );

  let json: { result: ListSubscriberLogsResponse; error?: string };
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

export async function clearSubscriberLogs(authToken: string): Promise<void> {
  const response = await fetch(`/api/v1/logs/subscriber`, {
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

export const getSubscriberLogRetentionPolicy = async (authToken: string) => {
  const response = await fetch(`/api/v1/logs/subscriber/retention`, {
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

export const updateSubscriberLogRetentionPolicy = async (
  authToken: string,
  days: number,
) => {
  const response = await fetch(`/api/v1/logs/subscriber/retention`, {
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
