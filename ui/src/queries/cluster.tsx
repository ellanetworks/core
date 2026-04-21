import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type DrainState = "active" | "draining" | "drained";

export type ClusterMember = {
  nodeId: number;
  raftAddress: string;
  apiAddress: string;
  binaryVersion: string;
  suffrage: "voter" | "nonvoter";
  maxSchemaVersion: number;
  isLeader: boolean;
  drainState: DrainState;
  drainUpdatedAt?: string;
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
  deadlineSeconds?: number;
};

export type DrainResponse = {
  message: string;
  state: DrainState;
  transferredLeadership: boolean;
  ransNotified: number;
  bgpStopped: boolean;
  sessionsRemaining: number;
};

export type ResumeResponse = {
  message: string;
  state: DrainState;
  bgpStarted: boolean;
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
  force = false,
): Promise<void> {
  const query = force ? "?force=true" : "";
  await apiFetchVoid(`/api/v1/cluster/members/${nodeId}${query}`, {
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

export async function drainClusterMember(
  authToken: string,
  nodeId: number,
  opts: DrainOptions = {},
): Promise<DrainResponse> {
  return apiFetch<DrainResponse>(`/api/v1/cluster/members/${nodeId}/drain`, {
    method: "POST",
    authToken,
    body: opts,
  });
}

export async function resumeClusterMember(
  authToken: string,
  nodeId: number,
): Promise<ResumeResponse> {
  return apiFetch<ResumeResponse>(`/api/v1/cluster/members/${nodeId}/resume`, {
    method: "POST",
    authToken,
  });
}

export async function getAutopilotState(
  authToken: string,
): Promise<AutopilotState> {
  return apiFetch<AutopilotState>("/api/v1/cluster/autopilot", { authToken });
}

export type MintJoinTokenParams = {
  nodeID: number;
  ttlSeconds?: number;
};

export type MintJoinTokenResponse = {
  token: string;
  expiresAt: number;
};

export async function mintClusterJoinToken(
  authToken: string,
  params: MintJoinTokenParams,
): Promise<MintJoinTokenResponse> {
  return apiFetch<MintJoinTokenResponse>("/api/v1/cluster/pki/join-tokens", {
    method: "POST",
    authToken,
    body: params,
  });
}

export type PKICertSummary = {
  fingerprint: string;
  status: string;
  notAfter?: number;
  hasCrossSigned: boolean;
};

export type ClusterPKIState = {
  clusterID: string;
  roots: PKICertSummary[];
  intermediates: PKICertSummary[];
  issuedCertSerialsByNode: Record<string, number[]>;
  revokedSerialCount: number;
};

export async function getClusterPKIState(
  authToken: string,
): Promise<ClusterPKIState> {
  return apiFetch<ClusterPKIState>("/api/v1/cluster/pki/state", { authToken });
}
