import { apiFetch, apiFetchVoid } from "@/queries/utils";

export type APISlice = {
  name: string;
  sst: number;
  sd: string;
};

export type ListSlicesResponse = {
  items: APISlice[];
  page: number;
  per_page: number;
  total_count: number;
};

export async function listSlices(
  authToken: string,
  page: number,
  perPage: number,
): Promise<ListSlicesResponse> {
  return apiFetch<ListSlicesResponse>(
    `/api/v1/slices?page=${page}&per_page=${perPage}`,
    { authToken },
  );
}

export async function getSlice(
  authToken: string,
  name: string,
): Promise<APISlice> {
  return apiFetch<APISlice>(`/api/v1/slices/${encodeURIComponent(name)}`, {
    authToken,
  });
}

export const createSlice = async (
  authToken: string,
  name: string,
  sst: number,
  sd: string,
): Promise<void> => {
  const body: Record<string, unknown> = { name, sst };
  if (sd) {
    body.sd = sd;
  }
  await apiFetchVoid(`/api/v1/slices`, {
    method: "POST",
    authToken,
    body,
  });
};

export const updateSlice = async (
  authToken: string,
  name: string,
  sst: number,
  sd: string,
): Promise<void> => {
  const body: Record<string, unknown> = { sst };
  if (sd) {
    body.sd = sd;
  }
  await apiFetchVoid(`/api/v1/slices/${encodeURIComponent(name)}`, {
    method: "PUT",
    authToken,
    body,
  });
};

export const deleteSlice = async (
  authToken: string,
  name: string,
): Promise<void> => {
  await apiFetchVoid(`/api/v1/slices/${encodeURIComponent(name)}`, {
    method: "DELETE",
    authToken,
  });
};
