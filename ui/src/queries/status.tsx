import { apiFetch } from "@/queries/utils";

export type APIStatus = {
  initialized: boolean;
  version?: string;
};

export const getStatus = async (): Promise<APIStatus> => {
  return apiFetch<APIStatus>(`/api/v1/status`);
};
