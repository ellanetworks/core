// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

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
  limit?: number,
): Promise<UsageResult> {
  const params = new URLSearchParams({ start, end });

  if (subscriber.trim() !== "") {
    params.set("subscriber", subscriber);
  }

  if (limit !== undefined) {
    params.set("limit", String(limit));
  }

  return apiFetch<UsageResult>(
    `/api/v1/subscriber-usage?group_by=${groupBy}&${params.toString()}`,
    { authToken },
  );
}

export async function clearUsageData(authToken: string): Promise<void> {
  return apiFetchVoid(`/api/v1/subscriber-usage`, {
    method: "DELETE",
    authToken,
  });
}

export const getUsageRetentionPolicy = async (
  authToken: string,
): Promise<UsageRetentionPolicy> => {
  return apiFetch<UsageRetentionPolicy>(`/api/v1/subscriber-usage/retention`, {
    authToken,
  });
};

export const updateUsageRetentionPolicy = async (
  authToken: string,
  days: number,
) => {
  return apiFetchVoid(`/api/v1/subscriber-usage/retention`, {
    method: "PUT",
    authToken,
    body: { days },
  });
};
