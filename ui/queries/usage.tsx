import { HTTPStatus } from "@/queries/utils";

export type SubscriberUsage = {
  uplink_bytes: number;
  downlink_bytes: number;
  total_bytes: number;
};

export type UsageResult = Array<Record<string, SubscriberUsage>>;

export type UsageRetentionPolicy = {
  days: number;
};

export async function getUsage(
  authToken: string,
  start: string,
  end: string,
  subscriber: string,
  groupBy: "day" | "subscriber",
): Promise<UsageResult> {
  const params = new URLSearchParams({
    start,
    end,
  });

  if (subscriber.trim() !== "") {
    params.set("subscriber", subscriber);
  }

  const response = await fetch(
    `/api/v1/subscriber-usage?group_by=${groupBy}&${params.toString()}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + authToken,
      },
    },
  );

  let json: { result: UsageResult; error?: string };
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

export const getUsageRetentionPolicy = async (authToken: string) => {
  const response = await fetch(`/api/v1/subscriber-usage/retention`, {
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

export const updateUsageRetentionPolicy = async (
  authToken: string,
  days: number,
) => {
  const response = await fetch(`/api/v1/subscriber-usage/retention`, {
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
