import { apiFetch } from "@/queries/utils";

export type ListSlicesResponse = {
  items: { name: string; sst: number; sd: string }[];
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
