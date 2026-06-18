// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type APIProfile = {
  name: string;
  ue_ambr_uplink: string;
  ue_ambr_downlink: string;
  allow_4g: boolean;
  allow_5g: boolean;
};

export type ListProfilesResponse = {
  items: APIProfile[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listProfiles(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListProfilesResponse> {
  return apiFetch<ListProfilesResponse>(
    `/api/v1/profiles?page=${page}&per_page=${perPage}`,
    { authToken },
  );
}

export async function getProfile(
  authToken: string,
  name: string,
): Promise<APIProfile> {
  return apiFetch<APIProfile>(`/api/v1/profiles/${encodeURIComponent(name)}`, {
    authToken,
  });
}

export async function createProfile(
  authToken: string,
  name: string,
  ueAmbrUplink: string,
  ueAmbrDownlink: string,
  allow4g: boolean,
  allow5g: boolean,
): Promise<void> {
  await apiFetchVoid(`/api/v1/profiles`, {
    method: "POST",
    authToken,
    body: {
      name,
      ue_ambr_uplink: ueAmbrUplink,
      ue_ambr_downlink: ueAmbrDownlink,
      allow_4g: allow4g,
      allow_5g: allow5g,
    },
  });
}

export async function updateProfile(
  authToken: string,
  name: string,
  ueAmbrUplink: string,
  ueAmbrDownlink: string,
  allow4g: boolean,
  allow5g: boolean,
): Promise<void> {
  await apiFetchVoid(`/api/v1/profiles/${encodeURIComponent(name)}`, {
    method: "PUT",
    authToken,
    body: {
      ue_ambr_uplink: ueAmbrUplink,
      ue_ambr_downlink: ueAmbrDownlink,
      allow_4g: allow4g,
      allow_5g: allow5g,
    },
  });
}

export async function deleteProfile(
  authToken: string,
  name: string,
): Promise<void> {
  await apiFetchVoid(`/api/v1/profiles/${encodeURIComponent(name)}`, {
    method: "DELETE",
    authToken,
  });
}
