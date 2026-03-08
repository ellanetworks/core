import { apiFetch, apiFetchVoid } from "@/queries/utils";

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

export type AuditLogFilters = {
  start?: string;
  end?: string;
  actor?: string;
};

export async function listAuditLogs(
  authToken: string,
  page: number,
  perPage: number,
  params?: AuditLogFilters,
): Promise<ListAuditLogsResponse> {
  const url = new URL(`/api/v1/logs/audit`, window.location.origin);
  url.searchParams.set("page", String(page));
  url.searchParams.set("per_page", String(perPage));

  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== "") {
        url.searchParams.set(k, v);
      }
    }
  }

  return apiFetch<ListAuditLogsResponse>(url.toString(), { authToken });
}

export const getAuditLogRetentionPolicy = async (
  authToken: string,
): Promise<AuditLogRetentionPolicy> => {
  return apiFetch<AuditLogRetentionPolicy>(`/api/v1/logs/audit/retention`, {
    authToken,
  });
};

export const updateAuditLogRetentionPolicy = async (
  authToken: string,
  days: number,
) => {
  return apiFetchVoid(`/api/v1/logs/audit/retention`, {
    method: "PUT",
    authToken,
    body: { days },
  });
};
