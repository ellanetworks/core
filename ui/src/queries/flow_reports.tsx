import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type FlowReport = {
  id: number;
  subscriber_id: string;
  source_ip: string;
  destination_ip: string;
  source_port: number;
  destination_port: number;
  protocol: number;
  packets: number;
  bytes: number;
  start_time: string;
  end_time: string;
  direction: string;
};

export type ListFlowReportsResponse = {
  items: FlowReport[];
  page: number;
  per_page: number;
  total_count: number;
};

export type FlowReportsRetentionPolicy = {
  days: number;
};

export async function listFlowReports(
  authToken: string,
  page: number,
  perPage: number,
  params?: Record<string, string>,
): Promise<ListFlowReportsResponse> {
  const url = new URL(`/api/v1/flow-reports`, window.location.origin);
  url.searchParams.set("page", String(page));
  url.searchParams.set("per_page", String(perPage));

  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v !== "") {
        url.searchParams.set(k, v);
      }
    }
  }

  return apiFetch<ListFlowReportsResponse>(url.toString(), { authToken });
}

export async function clearFlowReports(authToken: string): Promise<void> {
  return apiFetchVoid(`/api/v1/flow-reports`, {
    method: "DELETE",
    authToken,
  });
}

export const getFlowReportsRetentionPolicy = async (
  authToken: string,
): Promise<FlowReportsRetentionPolicy> => {
  return apiFetch<FlowReportsRetentionPolicy>(
    `/api/v1/flow-reports/retention`,
    { authToken },
  );
};

export const updateFlowReportsRetentionPolicy = async (
  authToken: string,
  days: number,
): Promise<void> => {
  return apiFetchVoid(`/api/v1/flow-reports/retention`, {
    method: "PUT",
    authToken,
    body: { days },
  });
};

export type FlowReportProtocolStat = {
  protocol: number;
  count: number;
};

export type FlowReportIPStat = {
  ip: string;
  count: number;
};

export type FlowReportStatsResponse = {
  protocols: FlowReportProtocolStat[];
  top_sources: FlowReportIPStat[];
  top_destinations: FlowReportIPStat[];
};

export async function getFlowReportStats(
  authToken: string,
  params?: Record<string, string>,
): Promise<FlowReportStatsResponse> {
  const url = new URL(`/api/v1/flow-reports/stats`, window.location.origin);

  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v !== "") {
        url.searchParams.set(k, v);
      }
    }
  }

  return apiFetch<FlowReportStatsResponse>(url.toString(), { authToken });
}
