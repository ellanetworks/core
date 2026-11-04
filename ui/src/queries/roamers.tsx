// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";
import type {
  Registration,
  SessionInfo,
  SubscriberListStatus,
} from "@/queries/subscribers";

export type HomePLMN = { mcc: string; mnc: string };

export type APIRoamerSummary = {
  imsi: string;
  partner: string;
  home_plmn?: HomePLMN;
  msisdn?: string;
  since?: string;
  status: SubscriberListStatus;
};

export type ListRoamersResponse = {
  items: APIRoamerSummary[];
  page: number;
  per_page: number;
  total_count: number;
};

export type APIRoamer = {
  imsi: string;
  partner: string;
  home_plmn?: HomePLMN;
  msisdn?: string;
  since?: string;
  registrations: Registration[];
  sessions: SessionInfo[];
};

export async function listRoamers(
  authToken: string,
  page: number,
  perPage: number,
  search?: string,
  partner?: string,
): Promise<ListRoamersResponse> {
  const params = new URLSearchParams({
    page: String(page),
    per_page: String(perPage),
  });

  if (search) params.set("search", search);
  if (partner) params.set("partner", partner);

  return apiFetch<ListRoamersResponse>(`/api/v1/roamers?${params.toString()}`, {
    authToken,
  });
}

const ROSTER_PER_PAGE = 100;

const ROAMER_ROSTER_CAP = 1000;

export async function listRoamerPartners(
  authToken: string,
): Promise<Record<string, string>> {
  const first = await listRoamers(authToken, 1, ROSTER_PER_PAGE);
  const items = first.items ?? [];
  const totalCount = Math.min(
    first.total_count ?? items.length,
    ROAMER_ROSTER_CAP,
  );

  const pageCount = Math.ceil(totalCount / ROSTER_PER_PAGE);
  const rest =
    pageCount > 1
      ? await Promise.all(
          Array.from({ length: pageCount - 1 }, (_, i) =>
            listRoamers(authToken, i + 2, ROSTER_PER_PAGE),
          ),
        )
      : [];

  const roamers = items
    .concat(...rest.map((r) => r.items ?? []))
    .slice(0, ROAMER_ROSTER_CAP);

  return Object.fromEntries(roamers.map((r) => [r.imsi, r.partner]));
}

export const getRoamer = async (
  authToken: string,
  imsi: string,
): Promise<APIRoamer> =>
  apiFetch<APIRoamer>(`/api/v1/roamers/${imsi}`, { authToken });

export const deregisterRoamer = async (
  authToken: string,
  imsi: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/roamers/${imsi}`, {
    method: "DELETE",
    authToken,
  });
};
