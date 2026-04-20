import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type ClusterMember = {
  nodeId: number;
  raftAddress: string;
  apiAddress: string;
  binaryVersion: string;
  suffrage: "voter" | "nonvoter";
  maxSchemaVersion: number;
  isLeader: boolean;
};

export type AutopilotServer = {
  nodeId: number;
  raftAddress: string;
  nodeStatus: string;
  healthy: boolean;
  isLeader: boolean;
  hasVotingRights: boolean;
  lastContactMs: number;
  lastTerm: number;
  lastIndex: number;
  stableSince?: string;
};

export type AutopilotState = {
  healthy: boolean;
  failureTolerance: number;
  leaderNodeId: number;
  voters: number[];
  servers: AutopilotServer[];
};

export type AddClusterMemberParams = {
  nodeId: number;
  raftAddress: string;
  apiAddress: string;
  suffrage?: "voter" | "nonvoter";
};

export type DrainOptions = {
  timeoutSeconds?: number;
};

export type DrainResponse = {
  message: string;
  transferredLeadership: boolean;
  ransNotified: number;
  bgpStopped: boolean;
};

export async function listClusterMembers(
  authToken: string,
): Promise<ClusterMember[]> {
  return apiFetch<ClusterMember[]>("/api/v1/cluster/members", { authToken });
}

export async function getClusterMember(
  authToken: string,
  nodeId: number,
): Promise<ClusterMember> {
  return apiFetch<ClusterMember>(`/api/v1/cluster/members/${nodeId}`, {
    authToken,
  });
}

export async function addClusterMember(
  authToken: string,
  params: AddClusterMemberParams,
): Promise<void> {
  await apiFetchVoid("/api/v1/cluster/members", {
    method: "POST",
    authToken,
    body: params,
  });
}

export async function removeClusterMember(
  authToken: string,
  nodeId: number,
): Promise<void> {
  await apiFetchVoid(`/api/v1/cluster/members/${nodeId}`, {
    method: "DELETE",
    authToken,
  });
}

export async function promoteClusterMember(
  authToken: string,
  nodeId: number,
): Promise<void> {
  await apiFetchVoid(`/api/v1/cluster/members/${nodeId}/promote`, {
    method: "POST",
    authToken,
  });
}

export async function drainNode(
  authToken: string,
  opts: DrainOptions = {},
): Promise<DrainResponse> {
  return apiFetch<DrainResponse>("/api/v1/cluster/drain", {
    method: "POST",
    authToken,
    body: opts,
  });
}

export async function getAutopilotState(
  authToken: string,
): Promise<AutopilotState> {
  return apiFetch<AutopilotState>("/api/v1/cluster/autopilot", { authToken });
}
