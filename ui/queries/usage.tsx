import { HTTPStatus } from "@/queries/utils";

export type SubscriberUsage = {
  uplink_bytes: number;
  downlink_bytes: number;
  total_bytes: number;
};

export type UsagePerDayResult = Array<Record<string, SubscriberUsage>>;

export async function getUsagePerDay(
  authToken: string,
  start: string,
  end: string,
  subscriber: string,
): Promise<UsagePerDayResult> {
  const params = new URLSearchParams({
    start,
    end,
  });

  if (subscriber.trim() !== "") {
    params.set("subscriber", subscriber);
  }

  const response = await fetch(
    `/api/v1/subscriber-usage/per-day?${params.toString()}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + authToken,
      },
    },
  );

  let json: { result: UsagePerDayResult; error?: string };
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

export type UsagePerSubscriberResult = Array<Record<string, SubscriberUsage>>;

export async function getUsagePerSubscriber(
  authToken: string,
  start: string,
  end: string,
  subscriber: string,
): Promise<UsagePerSubscriberResult> {
  const params = new URLSearchParams({
    start,
    end,
  });

  if (subscriber.trim() !== "") {
    params.set("subscriber", subscriber);
  }

  const response = await fetch(
    `/api/v1/subscriber-usage/per-subscriber?${params.toString()}`,
    {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Authorization: "Bearer " + authToken,
      },
    },
  );

  let json: { result: UsagePerSubscriberResult; error?: string };
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
