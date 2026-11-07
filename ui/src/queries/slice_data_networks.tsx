// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type APISliceDataNetwork = {
  data_network: string;
  nat: boolean;
  local_switch: boolean;
  network_instance: string;
};

export type ListSliceDataNetworksResponse = {
  items: APISliceDataNetwork[];
};

const basePath = (slice: string) =>
  `/api/v1/slices/${encodeURIComponent(slice)}/data-networks`;

export async function listSliceDataNetworks(
  authToken: string,
  slice: string,
): Promise<ListSliceDataNetworksResponse> {
  return apiFetch<ListSliceDataNetworksResponse>(basePath(slice), {
    authToken,
  });
}

export const createSliceDataNetwork = async (
  authToken: string,
  slice: string,
  dataNetwork: string,
  nat: boolean,
  localSwitch: boolean,
): Promise<void> => {
  await apiFetchVoid(basePath(slice), {
    method: "POST",
    authToken,
    body: { data_network: dataNetwork, nat, local_switch: localSwitch },
  });
};

export const updateSliceDataNetwork = async (
  authToken: string,
  slice: string,
  dataNetwork: string,
  settings: {
    nat?: boolean;
    local_switch?: boolean;
    network_instance?: string;
  },
): Promise<void> => {
  await apiFetchVoid(`${basePath(slice)}/${encodeURIComponent(dataNetwork)}`, {
    method: "PUT",
    authToken,
    body: settings,
  });
};

export const deleteSliceDataNetwork = async (
  authToken: string,
  slice: string,
  dataNetwork: string,
): Promise<void> => {
  await apiFetchVoid(`${basePath(slice)}/${encodeURIComponent(dataNetwork)}`, {
    method: "DELETE",
    authToken,
  });
};
