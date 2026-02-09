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

export async function listAuditLogs(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListAuditLogsResponse> {
  return apiFetch<ListAuditLogsResponse>(
    `/api/v1/logs/audit?page=${page}&per_page=${perPage}`,
    { authToken },
  );
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
