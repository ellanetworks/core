// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type PlmnID = {
  mcc: string;
  mnc: string;
};

export type Tai = {
  plmnID: PlmnID;
  tac: string;
};

export type Snssai = {
  sst: number;
  sd: string;
};

export type SupportedTAI = {
  tai: Tai;
  snssais: Snssai[];
};

export type RadioStatus = "online" | "offline";

export type APIRadio = {
  name: string;
  ref: string;
  id: string;
  plmn?: PlmnID;
  bit_length?: number;
  address: string;
  type: string;
  status: RadioStatus;
  connected_at: string;
  last_seen_at: string;
  disconnected_at: string;
  supported_tais: SupportedTAI[];
};

export type ListRadiosResponse = {
  items: APIRadio[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listRadios(
  authToken: string,
  page: number,
  perPage: number,
  status?: RadioStatus,
): Promise<ListRadiosResponse> {
  const query = new URLSearchParams({
    page: String(page),
    per_page: String(perPage),
  });
  if (status) query.set("status", status);

  return apiFetch<ListRadiosResponse>(`/api/v1/ran/radios?${query}`, {
    authToken,
  });
}

export type APIRadioDetail = {
  name: string;
  ref: string;
  id: string;
  plmn?: PlmnID;
  bit_length?: number;
  address: string;
  status: RadioStatus;
  connected_at: string;
  last_seen_at: string;
  disconnected_at: string;
  type: string;
  supported_tais: SupportedTAI[];
};

export function radioPath(ref: string): string {
  return encodeURIComponent(ref);
}

export async function getRadio(
  authToken: string,
  ref: string,
): Promise<APIRadioDetail> {
  return apiFetch<APIRadioDetail>(`/api/v1/ran/radios/${radioPath(ref)}`, {
    authToken,
  });
}

export async function forgetRadio(
  authToken: string,
  ref: string,
): Promise<void> {
  await apiFetchVoid(`/api/v1/ran/radios/${radioPath(ref)}`, {
    method: "DELETE",
    authToken,
  });
}

export async function findRadioByName(
  authToken: string,
  name: string,
): Promise<APIRadio | null> {
  const perPage = 100;

  let best: APIRadio | null = null;

  for (let page = 1; ; page++) {
    const response = await listRadios(authToken, page, perPage);

    for (const radio of response.items) {
      if (radio.name !== name) continue;
      if (radio.status === "online") return radio;

      best ??= radio;
    }

    if (page * perPage >= response.total_count || response.items.length === 0) {
      return best;
    }
  }
}
