import { HTTPStatus } from "@/queries/utils";

export type RadioLogRetentionPolicy = {
  days: number;
};

export type APIRadioLog = {
  id: number;
  timestamp: string;
  ran_id: string;
  event: string;
  details?: string;
};

export type ListRadioLogsResponse = {
  items: APIRadioLog[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listRadioLogs(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListRadioLogsResponse> {
  const response = await fetch(
    `/api/v1/logs/radio?page=${page}&per_page=${perPage}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${authToken}`,
      },
    },
  );

  let json: { result: ListRadioLogsResponse; error?: string };
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

export const getRadioLogRetentionPolicy = async (authToken: string) => {
  const response = await fetch(`/api/v1/logs/radio/retention`, {
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

export const updateRadioLogRetentionPolicy = async (
  authToken: string,
  days: number,
) => {
  const response = await fetch(`/api/v1/logs/radio/retention`, {
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
