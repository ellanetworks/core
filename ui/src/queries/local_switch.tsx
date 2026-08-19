// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type LocalSwitchInfo = {
  enabled: boolean;
};

export const getLocalSwitchInfo = async (
  authToken: string,
): Promise<LocalSwitchInfo> => {
  return apiFetch<LocalSwitchInfo>(`/api/v1/networking/local-switch`, {
    authToken,
  });
};

export const updateLocalSwitchInfo = async (
  authToken: string,
  enabled: boolean,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/networking/local-switch`, {
    method: "PUT",
    authToken,
    body: { enabled },
  });
};
