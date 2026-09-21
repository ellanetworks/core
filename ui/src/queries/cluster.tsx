// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { ApiError, apiFetch, apiFetchVoid } from "@/queries/utils";
import { NodeId } from "@/queries/nodeId";

export type DrainState = "active" | "draining" | "drained";

export type ClusterMember = {
  nodeId: NodeId;
  displayName: string;
  amfPointer: number;
  raftAddress: string;
  apiAddress: string;
  binaryVersion: string;
  suffrage: "voter" | "nonvoter";
  isLeader: boolean;
  drainState: DrainState;
  drainUpdatedAt?: string;
};

export type AutopilotServer = {
  nodeId: NodeId;
  raftAddress: string;
  nodeStatus: string;
  healthy: boolean;
  isLeader: boolean;
  hasVotingRights: boolean;
  stableSince?: string;
};

export type AutopilotState = {
  healthy: boolean;
  failureTolerance: number;
  leaderNodeId: NodeId;
  voters: NodeId[];
  servers: AutopilotServer[];
};

export type DrainResponse = {
  drainState: DrainState;
};

export async function listClusterMembers(
  authToken: string,
): Promise<ClusterMember[]> {
  return apiFetch<ClusterMember[]>("/api/v1/cluster/members", { authToken });
}

export async function removeClusterMember(
  authToken: string,
  nodeId: NodeId,
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
  nodeId: NodeId,
): Promise<void> {
  await apiFetchVoid(`/api/v1/cluster/members/${nodeId}/promote`, {
    method: "POST",
    authToken,
  });
}

export async function drainClusterMember(
  authToken: string,
  nodeId: NodeId,
): Promise<DrainResponse> {
  return apiFetch<DrainResponse>(`/api/v1/cluster/members/${nodeId}/drain`, {
    method: "POST",
    authToken,
  });
}

export async function resumeClusterMember(
  authToken: string,
  nodeId: NodeId,
): Promise<void> {
  await apiFetchVoid(`/api/v1/cluster/members/${nodeId}/resume`, {
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

export async function setClusterMemberDisplayName(
  authToken: string,
  nodeId: NodeId,
  displayName: string,
): Promise<void> {
  await apiFetchVoid(`/api/v1/cluster/members/${nodeId}/display-name`, {
    method: "PUT",
    authToken,
    body: { displayName },
  });
}

export type ClusterJoinState = "unavailable" | "waiting" | "joining" | "joined";

export type ClusterJoinStatus = {
  state: ClusterJoinState;
  error?: string;
};

export async function getClusterJoinStatus(): Promise<ClusterJoinStatus> {
  return apiFetch<ClusterJoinStatus>("/api/v1/cluster/join", {});
}

export async function joinCluster(
  token: string,
  seedAddresses: string[],
): Promise<ClusterJoinStatus> {
  return apiFetch<ClusterJoinStatus>("/api/v1/cluster/join", {
    method: "POST",
    body: { token, seedAddresses },
  });
}

export async function bootstrapCluster(): Promise<ClusterJoinStatus> {
  return apiFetch<ClusterJoinStatus>("/api/v1/cluster/bootstrap", {
    method: "POST",
  });
}

export async function waitForClusterReady(
  timeoutMs = 60000,
  intervalMs = 500,
): Promise<void> {
  const deadline = Date.now() + timeoutMs;

  while (Date.now() < deadline) {
    try {
      const status = await getClusterJoinStatus();
      if (status.state === "joined") return;
      if (status.state === "waiting" && status.error) {
        throw new Error(status.error);
      }
    } catch (err) {
      if (!(err instanceof ApiError) || !err.retryable) throw err;
    }

    await new Promise((resolve) => setTimeout(resolve, intervalMs));
  }

  throw new Error("The cluster did not finish forming in time");
}
