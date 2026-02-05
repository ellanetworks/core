import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type RadioEventRetentionPolicy = {
  days: number;
};

export type APIRadioEvent = {
  id: number;
  timestamp: string;
  protocol: string;
  message_type: string;
  direction: string;
  local_address: string;
  remote_address: string;
  details?: string;
};

export type ListRadioEventsResponse = {
  items: APIRadioEvent[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listRadioEvents(
  authToken: string,
  page: number,
  perPage: number,
  params?: Record<string, string | string[]>,
): Promise<ListRadioEventsResponse> {
  const url = new URL(`/api/v1/ran/events`, window.location.origin);
  url.searchParams.set("page", String(page));
  url.searchParams.set("per_page", String(perPage));

  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (Array.isArray(v)) url.searchParams.append(k, v.join(","));
      else url.searchParams.append(k, v);
    }
  }

  return apiFetch<ListRadioEventsResponse>(url.toString(), { authToken });
}

export type EnumField = {
  label: string;
  value: number;
  type: "enum";
};

export type DecodedRadioEvent = {
  pdu_type: string;
  message_type: string;
  procedure_code: EnumField;
  criticality: EnumField;
  value: unknown;
};

export type RadioEventContent = {
  decoded: DecodedRadioEvent;
  raw: string;
};

export async function getRadioEvent(
  authToken: string,
  id: string,
): Promise<RadioEventContent> {
  const url = new URL(`/api/v1/ran/events/${id}`, window.location.origin);
  return apiFetch<RadioEventContent>(url.toString(), { authToken });
}

export async function clearRadioEvents(authToken: string): Promise<void> {
  return apiFetchVoid(`/api/v1/ran/events`, {
    method: "DELETE",
    authToken,
  });
}

export const getRadioEventRetentionPolicy = async (authToken: string): Promise<RadioEventRetentionPolicy> => {
  return apiFetch<RadioEventRetentionPolicy>(`/api/v1/ran/events/retention`, { authToken });
};

export const updateRadioEventRetentionPolicy = async (
  authToken: string,
  days: number,
) => {
  return apiFetchVoid(`/api/v1/ran/events/retention`, {
    method: "PUT",
    authToken,
    body: { days },
  });
};
