import { apiFetch } from "@/queries/utils";

export type APIFleetStatus = {
  managed: boolean;
  lastSyncAt?: string;
};

export type APIStatus = {
  initialized: boolean;
  version?: string;
  fleet: APIFleetStatus;
};

export const getStatus = async (): Promise<APIStatus> => {
  return apiFetch<APIStatus>(`/api/v1/status`);
};
