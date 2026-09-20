// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch } from "@/queries/utils";
import { NodeId } from "@/queries/nodeId";

export type PendingMigration = {
  currentSchema: number;
  targetSchema: number;
  laggardNodeId?: NodeId;
};

export type ClusterStatus =
  | {
      enabled: false;
      role?: undefined;
      nodeId?: undefined;
      displayName?: undefined;
      isLeader?: undefined;
      leaderNodeId?: undefined;
      leaderAPIAddress?: undefined;
      appliedIndex?: undefined;
      clusterId?: undefined;
      appliedSchemaVersion?: undefined;
      pendingMigration?: undefined;
    }
  | {
      enabled: true;
      role: string;
      nodeId: NodeId;
      displayName: string;
      isLeader: boolean;
      leaderNodeId: NodeId;
      leaderAPIAddress?: string;
      appliedIndex: number;
      clusterId?: string;
      appliedSchemaVersion: number;
      pendingMigration?: PendingMigration;
    };

export type APIStatus = {
  initialized: boolean;
  version?: string;
  revision?: string;
  ready?: boolean;
  schemaVersion?: number;
  cluster: ClusterStatus;
};

export const getStatus = async (): Promise<APIStatus> => {
  return apiFetch<APIStatus>(`/api/v1/status`);
};
