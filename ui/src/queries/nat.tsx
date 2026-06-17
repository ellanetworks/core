// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type NatInfo = {
  enabled: boolean;
};

export const getNATInfo = async (authToken: string): Promise<NatInfo> => {
  return apiFetch<NatInfo>(`/api/v1/networking/nat`, { authToken });
};

export const updateNATInfo = async (
  authToken: string,
  enabled: boolean,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/networking/nat`, {
    method: "PUT",
    authToken,
    body: { enabled },
  });
};
