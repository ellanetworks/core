import { apiFetch } from "@/queries/utils";

export type ClusterStatus = {
  enabled: boolean;
  role: string;
  nodeId: number;
  isLeader: boolean;
  leaderNodeId: number;
  leaderAPIAddress?: string;
  appliedIndex: number;
  clusterId?: string;
};

export type APIStatus = {
  initialized: boolean;
  version?: string;
  revision?: string;
  ready?: boolean;
  schemaVersion?: number;
  cluster?: ClusterStatus;
};

export const getStatus = async (): Promise<APIStatus> => {
  return apiFetch<APIStatus>(`/api/v1/status`);
};
