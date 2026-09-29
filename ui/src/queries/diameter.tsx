// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch } from "@/queries/utils";

export type DiameterPeerState =
  "down" | "connecting" | "open" | "suspect" | "reopen" | "closing";

export interface DiameterPeer {
  role: "smsc";
  host?: string;
  realm?: string;
  address: string;
  port: number;
  state: DiameterPeerState;
  since: string;
}

export interface DiameterStatus {
  host?: string;
  realm?: string;
  peers: DiameterPeer[];
}

export const getDiameterStatus = async (
  authToken: string,
): Promise<DiameterStatus> => {
  return apiFetch<DiameterStatus>(`/api/v1/networking/diameter`, {
    authToken,
  });
};
